package main

import "time"

const (
	dbDriver                       = "mysql"
	configJSONPath                 = "config.json"
	sqldbConnectionTimeout         = 4 * time.Second
	serviceAccountName             = "Utopia"
	limitMaxUserResponsesPerSecond = 1
	healthCheckTimeout             = time.Minute * 10
	waitAfterUtopiaReboot          = time.Second * 40
	logsPath                       = "debug.log"
	nicknameMaxLength              = 22
	limitWithdrawNotifyTimeout     = time.Minute * 2
	dialogFlowSessionID            = "123456789"

	comandBalance  = "баланс"
	comandBalance2 = "balance"
	comandManager  = "менеджер"

	testUserOnlinePubkey  = "07E7DDA00F179CDAD0A86881FA57D2E06962039BC2F04E2F5AB7B79D716ADA3C"
	journalLogsTimeFormat = "2006-01-02"

	gameVoucherTemplate        = "%s%s-%s-%s-%s"
	gameVoucherActivateTimeout = time.Minute * 10
	maxGameVoucherAmount       = 1000
)

var (
	gameVoucherLength int

	tips = []string{}

	tgEmojiList = []string{
		"😝", "😋", "🤩", "🤓", "😲", "🤐", "🧐", "🤪", "🙃", "🤑",
	}

	autoRebootDisabled bool
)
