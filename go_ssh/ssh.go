package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"os/user"
	"strings"
	"syscall"
	"time"

	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

type SSHTerminal struct {
	Session *ssh.Session
	exitMsg string
	stdout  io.Reader
	stdin   io.Writer
	stderr  io.Reader
}

type Server struct {
	Alias      string `json:"alias"`
	Address    string `json:"address"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
	UseKey     bool   `json:"use_key"`
}

type Config struct {
	Servers []Server `json:"servers"`
}

func loadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func getHomeDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return usr.HomeDir, nil
}

func connectToServer(server *Server) error {
	sshConfig := &ssh.ClientConfig{
		User:            server.User,
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// 使用密钥认证
	if server.UseKey {
		homeDir, err := getHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %v", err)
		}
		keyPath := strings.Replace(server.PrivateKey, "~", homeDir, 1)
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("failed to read private key %s: %v", keyPath, err)
		}
		privateKey, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return fmt.Errorf("failed to parse private key %s: %v", keyPath, err)
		}
		sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeys(privateKey))
	} else if server.Password != "" {
		sshConfig.Auth = append(sshConfig.Auth, ssh.Password(server.Password))
	}

	// 拼接地址和端口
	address := fmt.Sprintf("%s:%d", server.Address, server.Port)

	client, err := ssh.Dial("tcp", address, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to server %s: %v", address, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session on server %s: %v", address, err)
	}
	defer session.Close()

	s := SSHTerminal{Session: session}
	return s.interactiveSession()
}

func (t *SSHTerminal) updateTerminalSize() {
	go func() {
		// SIGWINCH is sent to the process when the window size of the terminal has changed.
		sigwinchCh := make(chan os.Signal, 1)
		signal.Notify(sigwinchCh, syscall.SIGWINCH)

		fd := int(os.Stdin.Fd())
		termWidth, termHeight, err := term.GetSize(fd)
		if err != nil {
			fmt.Println(err)
		}

		for {
			select {
			// The client updated the size of the local PTY. This change needs to occur
			// on the server side PTY as well.
			case sigwinch := <-sigwinchCh:
				if sigwinch == nil {
					return
				}
				currTermWidth, currTermHeight, err := term.GetSize(fd)

				// Terminal size has not changed, don't do anything.
				if currTermHeight == termHeight && currTermWidth == termWidth {
					continue
				}

				_ = t.Session.WindowChange(currTermHeight, currTermWidth)
				if err != nil {
					fmt.Printf("Unable to send window-change request: %s.", err)
					continue
				}

				termWidth, termHeight = currTermWidth, currTermHeight
			}
		}
	}()
}

func (t *SSHTerminal) interactiveSession() error {
	defer func() {
		if t.exitMsg == "" {
			fmt.Fprintln(os.Stdout, "the connection was closed on the remote side on ", time.Now().Format(time.RFC822))
		} else {
			fmt.Fprintln(os.Stdout, t.exitMsg)
		}
	}()

	fd := int(os.Stdin.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer term.Restore(fd, state)

	termWidth, termHeight, err := term.GetSize(fd)
	if err != nil {
		return err
	}

	termType := os.Getenv("TERM")
	if termType == "" {
		termType = "xterm-256color"
	}

	err = t.Session.RequestPty(termType, termHeight, termWidth, ssh.TerminalModes{})
	if err != nil {
		return err
	}

	t.updateTerminalSize()

	t.stdin, err = t.Session.StdinPipe()
	if err != nil {
		return err
	}
	t.stdout, err = t.Session.StdoutPipe()
	if err != nil {
		return err
	}
	t.stderr, err = t.Session.StderrPipe()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(os.Stderr, t.stderr)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(os.Stdout, t.stdout)
	}()

	// Handle user input
	go func() {
		buf := make([]byte, 128)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				fmt.Println(err)
				return
			}
			if n > 0 {
				_, err = t.stdin.Write(buf[:n])
				if err != nil {
					fmt.Println(err)
					t.exitMsg = err.Error()
					return
				}
			}
		}
	}()

	err = t.Session.Shell()
	if err != nil {
		return err
	}
	wg.Wait()
	err = t.Session.Wait()
	if err != nil {
		return err
	}
	return nil
}

func main() {
	// 只接收别名或 IP 地址参数，配置文件路径可以自定义
	configFile := flag.String("config", "config.json", "Path to the configuration file")
	aliasFlag := flag.String("alias", "", "Server alias to connect to")
	ipFlag := flag.String("ip", "", "IP address of the server to connect to")
	flag.Parse()

	// Load config file
	config, err := loadConfig(*configFile)
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}

	var selectedServer *Server

	// 如果有别名或 IP 地址参数，查找对应的服务器
	if *aliasFlag != "" {
		for _, server := range config.Servers {
			// if strings.ToLower(server.Alias) == strings.ToLower(*aliasFlag) {
			if strings.EqualFold(server.Alias, *aliasFlag) {
				selectedServer = &server
				break
			}
		}
	} else if *ipFlag != "" {
		for _, server := range config.Servers {
			if server.Address == *ipFlag {
				selectedServer = &server
				break
			}
		}
	}

	// 如果没有命令行参数，进入交互式选择
	if selectedServer == nil {
		fmt.Println("Please select a server to connect to:")
		for i, server := range config.Servers {
			fmt.Printf("%d. %s (%s:%d)\n", i+1, server.Alias, server.Address, server.Port)
		}
		var choice string
		_, _ = fmt.Scanln(&choice)
		choice = strings.TrimSpace(strings.ToLower(choice))
		for _, server := range config.Servers {
			if strings.ToLower(server.Alias) == choice {
				selectedServer = &server
				break
			}
		}
	}

	// 如果没有选择服务器，默认使用第一个
	if selectedServer == nil {
		selectedServer = &config.Servers[0]
	}

	// 连接所选服务器
	fmt.Printf("Connecting to %s (%s:%d)...\n", selectedServer.Alias, selectedServer.Address, selectedServer.Port)
	err = connectToServer(selectedServer)
	if err != nil {
		fmt.Println("Error:", err)
	}
}
