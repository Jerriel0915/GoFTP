package Constant

type Commands string

const (
	// HELP 获取帮助指令
	HELP = "help"

	// LOGIN 登录指令
	LOGIN = "login"

	// USR 用户名指令
	USR = "username"

	// PASS 密码指令
	PASS = "password"

	// PASV 被动模式指令
	PASV = "passive"

	// CWD 更改工作目录指令
	CWD = "cwd"

	// PWD 查看当前工作目录指令
	PWD = "pwd"
)
