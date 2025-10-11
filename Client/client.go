package main

import (
	"GoFTP/Constant"
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	CtrlPort = "21"
)

var dataConn net.Conn // 数据连接

func main() {
	conn, err := net.Dial("tcp", "127.0.0.1:"+CtrlPort)
	if err != nil {
		log.Println("Error connecting server:", err)
		os.Exit(1)
	}
	defer conn.Close()

	log.Println("Connected to server!")

	reader := bufio.NewReader(conn)
	go readServerResponses(reader)

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		fmt.Println(fields)

		if len(fields) == 0 {
			fmt.Print("> ")
			continue
		}

		command := strings.ToLower(fields[0]) // 命令内容
		args := fields[1:]                    // 参数

		switch command {
		case Constant.HELP:
			doHelp()
		case Constant.LOGIN:
			doLogin(conn)
		case Constant.PASV:
			doPASV(conn)
		case Constant.CWD:
			doCWD(conn, args)
		case Constant.PWD:
			doPWD(conn)
		}
		fmt.Print("> ")
	}

}

// 读取服务器消息
func readServerResponses(reader *bufio.Reader) {
	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			log.Println("Server disconnected or error reading:", err)
			return
		}
		fmt.Print("<- Server: " + msg)

		if strings.HasPrefix(msg, string(Constant.EnteringPassiveMode)) {
			ip, port, err := parsePASVResponse(msg)
			if err != nil {
				log.Println("Error parsing PASV response:", err)
			} else {
				dataConn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", ip, port))
				if err != nil {
					log.Println("Error establishing data connection:", err)
				} else {
					log.Println("Data connection established with", dataConn.RemoteAddr())
				}
			}
		}

		fmt.Print("> ")
	}
}

func sendToServer(conn net.Conn, messages ...string) {
	if len(messages) == 0 {
		return
	} else {
		_, err := fmt.Fprint(conn, strings.Join(messages, " ")+"\r\n")
		if err != nil {
			log.Printf("Error sent message to server! " + err.Error())
		}
	}
	// 刷新
	if flusher, ok := conn.(interface{ Flush() error }); ok {
		_ = flusher.Flush()
	} else {
		// 关闭 Nagle
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			_ = tcpConn.SetNoDelay(true)
		}
	}
}

// help
func doHelp() {

}

// login
func doLogin(conn net.Conn) {
	sendToServer(conn, Constant.LOGIN)

	doUSR(conn)
	doPASS(conn)
}

func doUSR(conn net.Conn) {
	var username string

	fmt.Scanln(&username)
	sendToServer(conn, Constant.USR, username)
}

func doPASS(conn net.Conn) {
	var password string

	fmt.Scanln(&password)
	sendToServer(conn, Constant.PASS, password)
}

func doPASV(conn net.Conn) {
	sendToServer(conn, Constant.PASV)
}

func doCWD(conn net.Conn, args []string) {
	if len(args) != 1 || len(args[0]) == 0 {
		log.Println("Arguments valid, usage: cwd <dir_path>")
		return
	}
	sendToServer(conn, Constant.CWD, args[0])
}

func doPWD(conn net.Conn) {
	sendToServer(conn, Constant.PWD)
}

// 解析服务端PASV地址
func parsePASVResponse(msg string) (string, int, error) {
	start := strings.Index(msg, "(")
	end := strings.Index(msg, ")")
	if start == -1 || end == -1 {
		return "", 0, errors.New("invalid PASV response format")
	}

	parts := strings.Split(msg[start+1:end], ",")
	if len(parts) != 6 {
		return "", 0, errors.New("invalid PASV response format")
	}

	ip := strings.Join(parts[:4], ".")

	p1, err := strconv.Atoi(parts[4])
	if err != nil {
		return "", 0, err
	}

	p2, err := strconv.Atoi(parts[5])
	if err != nil {
		return "", 0, err
	}

	port := p1*256 + p2

	return ip, port, nil
}
