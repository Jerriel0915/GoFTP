package constant

type Commands string

const (
	// HELP 查看指令用法
	HELP = "help"

	// LOGIN 登录
	LOGIN = "login"

	// USR 用户名
	USR = "username"

	// PASS 密码
	PASS = "password"

	// PASV 被动模式
	PASV = "passive"

	// CWD 更改工作目录
	CWD = "cwd"

	// PWD 查看当前工作目录
	PWD = "pwd"

	// LIST 获取子目录或文件列表
	LIST = "list"

	// STOR 上传文件
	STOR = "stor"

	// RETR 下载文件
	RETR = "retr"
)
