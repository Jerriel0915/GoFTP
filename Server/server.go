package main

import (
	"GoFTP/Constant"
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
)

const (
	CtrlPort      = "21"
	PASV_PORT_MIN = 1024
	PASV_PORT_MAX = 65535
)

type FTPConn struct {
	conn         net.Conn     // 连接控制
	dataConn     net.Conn     // 数据连接
	dataListener net.Listener // 数据监听
	rootDir      string       // 根目录
	workDir      string       // 工作目录

	username      string          // 用户名
	authorisation Constant.Status // 授权
}

func main() {
	// Create a root directory for the FTP server
	rootDir := "ftp_root"
	_, err := os.Stat(rootDir)
	if os.IsNotExist(err) {
		// directory not exist, create
		err := os.Mkdir(rootDir, 0755)

		if err != nil {
			log.Fatal(err)
			return
		}
	}

	// 创建控制端口，开启监听
	listen, err := net.Listen("tcp", ":"+CtrlPort)
	if err != nil {
		log.Println("Listen failed, err: ", err)
		return
	}
	defer listen.Close()
	log.Println("Listening on port " + CtrlPort)

	// 持续监听
	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Println("Accept failed, err: ", err)
			continue
		}
		log.Println("Accepted new connection from ", conn.RemoteAddr())

		// 新建FTP连接
		ftpConn := &FTPConn{
			conn:          conn,
			authorisation: Constant.NONE,
			rootDir:       rootDir,
			workDir:       "/"}
		go ftpConn.handleConnection()
	}
}

// 连接处理
func (c *FTPConn) handleConnection() {
	defer c.conn.Close()

	c.respond(Constant.ServiceReady, "Hello from FTP server!")

	scanner := bufio.NewScanner(c.conn)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		log.Println("<- Get from client: ", strings.Join(fields, " "))

		command := strings.ToLower(fields[0])
		args := fields[1:]

		ok, code, msg, err := c.solve(command, args)
		if !ok {
			log.Println("Command running failed, err: ", err)
		}
		c.respond(code, msg)
	}
}

func (c *FTPConn) solve(command string, args []string) (ok bool, code Constant.Code, msg string, err error) {
	switch command {
	case Constant.LOGIN:
		return c.handleLogin()
	case Constant.USR:
		return c.handleUsr(args)
	case Constant.PASS:
		return c.handlePASS(args)
	case Constant.PASV:
		return c.handlePASV()
	case Constant.CWD:
		return c.handleCWD(args)
	case Constant.PWD:
		return c.handlePWD()
	default:
		ok = false
		err = errors.New("command not recognized")
		return
	}
}

// 回应
func (c *FTPConn) respond(code Constant.Code, msg string) {
	response := string(code) + " | " + msg + "\r\n"
	_, err := fmt.Fprint(c.conn, response)
	if err != nil {
		log.Println("Respond failed, err: ", err)
	}

	log.Println("-> Response: " + response)
}

func (c *FTPConn) handleLogin() (ok bool, code Constant.Code, msg string, err error) {
	if c.authorisation != Constant.NONE {
		return false, Constant.CommandRunFail, "You have already login, username: " + c.username, nil
	}

	return true, Constant.NeedUsername, "Need username.", nil
}

func (c *FTPConn) handleUsr(args []string) (ok bool, code Constant.Code, msg string, err error) {
	if len(args) != 1 || len(args[0]) == 0 {
		return false, Constant.CommandArgsError, "Invalid number of arguments.", nil
	}

	username := args[0]
	c.username = username

	return true, Constant.NeedPassword, "Need password.", nil
}

func (c *FTPConn) handlePASS(args []string) (ok bool, code Constant.Code, msg string, err error) {
	if len(args) != 1 || len(args[0]) == 0 {
		return false, Constant.CommandArgsError, "Invalid number of arguments.", nil
	}

	if len(c.username) == 0 {
		return false, Constant.NeedUsername, "Need username.", nil
	}
	// TODO 接入数据库

	password := args[0]
	if c.username == "admin" && password == "123456" {
		c.authorisation = Constant.ADMIN
		return true, Constant.CommandRunSuccess, "Welcome! " + c.username, nil
	} else {
		return false, Constant.NotLogin, "Username or password error! Please retry", nil
	}
}

// 处理被动链接
func (c *FTPConn) handlePASV() (ok bool, code Constant.Code, msg string, err error) {
	if c.authorisation == Constant.NONE {
		return false, Constant.NotLogin, "You have not login.", nil
	}

	// 寻找可用端口
	port, err := findAvailablePort()
	if err != nil {
		return false, Constant.CannotOpenDataConnection, "Cannot open data connection.", err
	}

	// 获取服务器IP地址
	ip, err := getLocalIP()
	if err != nil {
		return false, Constant.CannotOpenDataConnection, "Cannot get local IP.", err
	}

	// 开启数据监听
	c.dataListener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false, Constant.CannotOpenDataConnection, "Cannot open data connection.", err
	}

	// 封装返回信息
	p1 := port / 256
	p2 := port % 256

	ipFields := strings.Split(ip.String(), ".")
	msg = fmt.Sprintf("Entering Passive Mode (%s,%s,%s,%s,%d,%d)", ipFields[0], ipFields[1], ipFields[2], ipFields[3], p1, p2)

	go func() {
		conn, err := c.dataListener.Accept()
		if err != nil {
			log.Println("Error accepting data connection:", err)
			return
		}
		c.dataConn = conn
		log.Println("Data connection established with", conn.RemoteAddr())
	}()

	return true, Constant.EnteringPassiveMode, msg, nil
}

// 处理 改变工作目录
func (c *FTPConn) handleCWD(args []string) (ok bool, code Constant.Code, msg string, err error) {
	if c.authorisation == Constant.NONE {
		return false, Constant.NotLogin, "You have not login.", nil
	}

	if len(args) != 1 || len(args[0]) == 0 {
		return false, Constant.CommandArgsError, "Invalid number of arguments.", nil
	}

	newDir := args[0]

	// 路径安全检查
	absPath, err := c.toAbsPath(newDir)
	if err != nil {
		return false, Constant.PathInvalid, err.Error(), err
	}

	// 检查是否存在对应目录
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, Constant.PathInvalid, "Directory does not exist.", err
		}
		return false, Constant.PathInvalid, "Error accessing path.", err
	}

	if !fileInfo.IsDir() {
		return false, Constant.PathInvalid, "Path is not a directory.", errors.New("path is not a directory")
	}

	// 基于当前用户根目录，更新工作目录
	var userRoot string
	if c.authorisation == Constant.ADMIN {
		userRoot, _ = filepath.Abs(c.rootDir)
	} else { // USER
		userRoot, _ = filepath.Abs(filepath.Join(c.rootDir, c.username))
	}

	newWorkDir, err := filepath.Rel(userRoot, absPath)
	if err != nil {
		return false, Constant.PathInvalid, "Error resolving relative path.", err
	}

	c.workDir = "/" + filepath.ToSlash(newWorkDir)
	if c.workDir == "/." {
		c.workDir = "/"
	}

	return true, Constant.FileCommandRunSuccess, "Directory changed successfully to " + c.workDir, nil
}

func (c *FTPConn) handlePWD() (ok bool, code Constant.Code, msg string, err error) {
	if c.authorisation == Constant.NONE {
		return false, Constant.NotLogin, "You have not login.", nil
	}
	return true, Constant.FileCommandRunSuccess, "You are now in " + c.workDir, nil
}

// toAbsPath 此方法将客户端提供的 path 转换为安全且绝对的服务端绝对路径，确保处于合法操作范围内
func (c *FTPConn) toAbsPath(path string) (string, error) {
	var userRoot string
	var err error

	// 根据职权判断
	switch c.authorisation {
	case Constant.ADMIN:
		userRoot, err = filepath.Abs(c.rootDir)
		if err != nil {
			return "", errors.New("cannot resolve server root directory")
		}
	case Constant.USER:
		userRoot = filepath.Join(c.rootDir, c.username)
		// 确保用户的根目录存在，如不存在则创建
		if _, err := os.Stat(userRoot); os.IsNotExist(err) {
			if err := os.MkdirAll(userRoot, 0755); err != nil {
				return "", errors.New("cannot create user directory")
			}
		}
		userRoot, err = filepath.Abs(userRoot)
		if err != nil {
			return "", errors.New("cannot resolve user root directory")
		}
	default:
		return "", errors.New("user not logged in")
	}

	var targetPath string
	// 若新路径以 “/” 开头，则视作从根目录开始
	// 若不是，则视作从当前工作目录开始
	if strings.HasPrefix(path, "/") {
		targetPath = filepath.Join(userRoot, path)
	} else {
		// Note: c.workDir is relative to the user's root.
		currentPath := filepath.Join(userRoot, c.workDir)
		targetPath = filepath.Join(currentPath, path)
	}

	// 处理 “..” 和 “.”
	cleanPath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", errors.New("error resolving path")
	}

	// 安全监测：确保最终路径处于合法范围内
	if !strings.HasPrefix(cleanPath, userRoot) {
		return "", errors.New("access denied: attempt to access outside of designated directory")
	}

	return cleanPath, nil
}

// 从 PASV_PORT_MIN 到 PASV_PORT_MAX 中选取一个可用的端口号并返回
func findAvailablePort() (port int, err error) {
	for port := PASV_PORT_MIN; port <= PASV_PORT_MAX; port++ {
		// 依次遍历，开启监听不报错即可用
		addr := fmt.Sprintf(":%d", port)
		l, err := net.Listen("tcp", addr)
		if err == nil {
			l.Close()
			return port, nil
		}
	}
	return 0, errors.New("no available port found")
}

// 获取本地IP地址
func getLocalIP() (net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP, nil
			}
		}
	}
	return nil, errors.New("no non-loopback IPv4 address found")
}
