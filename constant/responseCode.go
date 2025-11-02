package constant

type Code string

const (
	DataConnectionOpen    = "150"
	CommandRunSuccess     = "200"
	CommandRunFail        = "202"
	ServiceReady          = "220"
	ClosingDataConnection = "226"
	EnteringPassiveMode   = "227"
	FileCommandRunSuccess = "250"

	NeedPassword = "331"
	NeedUsername = "332"

	CannotOpenDataConnection = "425"
	TransferAborted          = "426"

	CommandNotDefine = "500"
	CommandArgsError = "501"
	NotLogin         = "530"
	NeedAccount      = "532"
	PathInvalid      = "550"
)
