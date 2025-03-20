# LinuxTools
A go/shell tool to connect server.

usage:

if go_ssh:

```shell
go run main.go --config=/home/config.json -alias server1
or:
go run main.go --config=/home/config.json -ip 192.168.0.200

if no args:
go run main.go --config=/home/config.json

result:
Please select a server to connect to:
1. server1 (192.168.0.200:22)
2. server2 (192.168.0.201:22)
3. server3 (192.168.0.202:22)
4. server4 (192.168.0.203:22)
Enter the alias of the server you want to connect to: server1
Connecting to 192.168.0.200:22...
```

or:

```shell
alias gossh='go run main.go --config=/home/config.json'

gossh -alias server1
gossh -ip 192.168.0.200

if not args:
gossh

result:
Please select a server to connect to:
1. server1 (192.168.0.200:22)
2. server2 (192.168.0.201:22)
3. server3 (192.168.0.202:22)
4. server4 (192.168.0.203:22)
Enter the alias of the server you want to connect to: server1
Connecting to 192.168.0.200:22...
```

if shell_ssh:

**You need install `jq` and `sshpass` tool**.

```shell
chmod +x ssh_connect.sh

# use args
./ssh_connect.sh server1

# interactive
./ssh_connect.sh

Please select a server to connect to:
server1 (192.168.1.100)
server2 (192.168.1.101)
Enter the alias or address of the server: server2

# Replace the path of your config.json.
CONFIG_FILE="/home/config.json"
```

You can alse use alias, like go_ssh.

```shell
alias issh='./ssh_connect.sh'

use args:
issh server1

interactive:
issh

Please select a server to connect to:
server1 (192.168.1.100)
server2 (192.168.1.101)
Enter the alias or address of the server: server2
```

