package main

import (
	"GoFTP/constant"
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	CtrlPort = ":21"

	DownloadPath = "./Downloads"
)

var dataConn net.Conn // 数据连接
var pasvReady = make(chan bool, 1)

func main() {
	var serverAddr string
	flag.StringVar(&serverAddr, "s", "", "Server address to connect to")
	flag.Parse()

	if serverAddr == "" {
		fmt.Println("Please input server public-ip: ")
		fmt.Scanln(&serverAddr)
	}

	conn, err := net.Dial("tcp", serverAddr+CtrlPort)

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
		case constant.HELP:
			doHelp()
		case constant.LOGIN:
			doLogin(conn)
		case constant.PASV:
			doPASV(conn)
		case constant.CWD:
			doCWD(conn, args)
		case constant.PWD:
			doPWD(conn)
		case constant.LIST:
			doLIST(conn, args)
		case constant.STOR:
			doSTOR(conn, args)
		case constant.RETR:
			doRETR(conn, args)
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

		if strings.HasPrefix(msg, string(constant.EnteringPassiveMode)) {
			ip, port, err := parsePASVResponse(msg)
			if err != nil {
				log.Println("Error parsing PASV response:", err)
			} else {
				dataConn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", ip, port))
				if err != nil {
					log.Println("Error establishing data connection:", err)
					pasvReady <- false
				} else {
					log.Println("Data connection established with", dataConn.RemoteAddr())
					pasvReady <- true
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
	sendToServer(conn, constant.LOGIN)

	doUSR(conn)
	doPASS(conn)
}

func doUSR(conn net.Conn) {
	var username string

	fmt.Scanln(&username)
	sendToServer(conn, constant.USR, username)
}

func doPASS(conn net.Conn) {
	var password string

	fmt.Scanln(&password)
	sendToServer(conn, constant.PASS, password)
}

func doPASV(conn net.Conn) {
	sendToServer(conn, constant.PASV)
}

func doCWD(conn net.Conn, args []string) {
	if len(args) != 1 || len(args[0]) == 0 {
		log.Println("Arguments valid, usage: cwd [dir_path]")
		return
	}
	sendToServer(conn, constant.CWD, args[0])
}

func doPWD(conn net.Conn) {
	sendToServer(conn, constant.PWD)
}

// args: [filePath] <limit> <page>
func doLIST(conn net.Conn, args []string) {
	// 1. 进入被动模式
	doPASV(conn)

	// 2. 等待数据连接
	if !<-pasvReady {
		log.Println("Failed to establish data connection.")
		return
	}
	defer dataConn.Close()
	// 重置全局数据连接
	defer func() { dataConn = nil }()

	// 3. 发送 LIST 指令
	switch len(args) {
	case 0:
		sendToServer(conn, constant.LIST, "/", "99", "0")
	case 1:
		if len(args[0]) == 0 {
			sendToServer(conn, constant.LIST, "/", "99", "0")
		}
		sendToServer(conn, constant.LIST, args[0], "99", "0")
	case 2:
		sendToServer(conn, constant.LIST, args[0], args[1], "0")
	case 3:
		sendToServer(conn, constant.LIST, args[0], args[1], args[2])
	default:
		log.Println("Arguments valid, usage: list [file_path] <limit> <page>")
		return
	}

	// 4. 读取数据
	listData, err := io.ReadAll(dataConn)
	if err != nil {
		log.Println("Error reading directory listing:", err)
		return
	}

	fmt.Println(string(listData))
}

func doSTOR(conn net.Conn, args []string) {
	if len(args) != 1 {
		log.Println("Usage: stor <local_file_path>")
		return
	}
	localPath := args[0]

	file, err := os.Open(localPath)
	if err != nil {
		log.Println("Error opening local file:", err)
		return
	}
	defer file.Close()

	// 1. 进入被动模式
	doPASV(conn)

	// 2. 等待数据连接
	if !<-pasvReady {
		log.Println("Failed to establish data connection.")
		return
	}
	defer dataConn.Close()
	// 重置全局数据连接
	defer func() { dataConn = nil }()

	// 3. 发送 STOR 指令
	remoteFileName := filepath.Base(localPath)
	sendToServer(conn, constant.STOR, remoteFileName)

	// 4. 发送正文
	n, err := io.Copy(dataConn, file)
	if err != nil {
		log.Println("Error sending file data:", err)
		return
	}
	log.Printf("%d bytes sent.", n)
}

func doRETR(conn net.Conn, args []string) {
	if len(args) != 1 {
		log.Println("Usage: stor <local_file_path>")
		return
	}

	_, err := os.Stat(DownloadPath)
	if os.IsNotExist(err) {
		// directory not exist, create
		err := os.Mkdir(DownloadPath, 0755)

		if err != nil {
			log.Println("Error creating download directory:", err)
			return
		}
	}

	targetFilePath := args[0]
	downloadFilePath := filepath.Join(DownloadPath, targetFilePath)

	// 1. 进入被动模式
	doPASV(conn)

	// 2. 等待数据连接
	if !<-pasvReady {
		log.Println("Failed to establish data connection.")
		return
	}
	defer dataConn.Close()
	// 重置全局数据连接
	defer func() { dataConn = nil }()

	// 3. 发送 RETR 指令
	sendToServer(conn, constant.RETR, targetFilePath)

	// 4. 文件重命名防止重复
	downloadFilePath, err = reNameFilePath(downloadFilePath)
	if err != nil {
		log.Println("Error renaming file:", err)
	}

	// 5. 创建新文件
	file, err := os.Create(downloadFilePath)
	if err != nil {
		log.Println("Error creating file:", err)
	}
	defer file.Close()

	// 6. 接收数据
	n, err := io.Copy(file, dataConn)
	if err != nil {
		log.Println("Error receiving file data:", err)
	}
	log.Printf("%d bytes received.", n)
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

func reNameFilePath(filePath string) (string, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return filePath, nil
	}

	dir := filepath.Dir(filePath)
	fileName := filepath.Base(filePath)
	ext := filepath.Ext(fileName)
	nameWithoutExt := fileName[:len(fileName)-len(ext)]

	// 检查文件名是否已经有数字后缀 (例如: file(1).txt, image-2.jpg)
	re := regexp.MustCompile(`^(.+?)(?:\((\d+)\)|-(\d+))$`)
	matches := re.FindStringSubmatch(nameWithoutExt)

	var baseName string
	var startNum int

	if len(matches) > 0 {
		// 如果已经有数字后缀，从下一个数字开始
		baseName = matches[1]
		if matches[2] != "" {
			startNum, _ = strconv.Atoi(matches[2])
		} else if matches[3] != "" {
			startNum, _ = strconv.Atoi(matches[3])
		}
		startNum++
	} else {
		// 没有数字后缀，从1开始
		baseName = nameWithoutExt
		startNum = 1
	}

	// 尝试不同的数字后缀直到找到可用的文件名
	for i := startNum; i < 1000; i++ { // 设置最大尝试次数防止无限循环
		newName := fmt.Sprintf("%s(%d)%s", baseName, i, ext)
		newPath := filepath.Join(dir, newName)

		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath, nil
		}
	}

	return "", errors.New("rename file failed")
}
