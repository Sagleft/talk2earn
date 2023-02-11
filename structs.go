package main

import (
	"database/sql"
	"sync"
	"time"

	tb "github.com/Sagleft/telegobot"
	utopiago "github.com/Sagleft/utopialib-go"
	"github.com/beefsack/go-rate"
	simplecron "github.com/sagleft/simple-cron"
)

type solution struct {
	DB                        *dbHandler
	TelegramBot               *tb.Bot
	Config                    config
	WsHandlers                map[string]wsHandler
	ContactsOnlineCache       []utopiago.ContactData
	ChannelOnlineCache        []utopiago.ChannelContactData
	WithdrawNotifyRateLimiter *rate.RateLimiter

	HandleContactsCron   *simplecron.CronObject
	VouchersGiveawayCron *simplecron.CronObject
	VouchersCooldown     map[string]time.Time // pubkey -> last time voucher activated

	IsContactsCheckInProgress bool
	UsersOnline               map[string]*onlineData
	UtopiaModerators          map[string]struct{} // pubkey -> empty struct
	TelegramModerators        map[int64]struct{}  // telegram ID -> empty struct

	MessageHandler   messagesHandler
	TelegramHandlers []handlerPair
}

type messagesHandler struct {
	sync.Mutex
	Client      *utopiago.UtopiaClient
	RateLimiter *rate.RateLimiter
}

type onlineData struct {
	Pubkey         string
	NotifySentOnce bool
}

type config struct {
	UtopiaCfg                utopiago.UtopiaClient `json:"utopia"`
	WelcomeMessages          []string              `json:"welcomeMessages"`
	InvalidMessage           string                `json:"invalidMessage"`
	ModeratorPubkeys         []string              `json:"moderatorPubkeys"`
	ModeratorTelegram        string                `json:"moderatorTelegram"`
	ModeratorTelegramIDs     []int64               `json:"moderatorTelegramIDs"`
	DB                       dbConnectionTask      `json:"db"`
	PointsNotifyDisabled     bool                  `json:"points_notify_disabled"`
	MinWithdraw              float64               `json:"min_withdraw"`
	PointsPer24h             float64               `json:"points_per_24h"`
	UseIntervals             bool                  `json:"use_intervals"`
	Intervals                []pointsInterval      `json:"intervals"`
	ContactsCronPerMinute    int                   `json:"per_minute_cron"`
	ChannelID                string                `json:"channel"`
	RequestsModeratorPubkey  string                `json:"requests_moderator_pubkey"`
	DialogflowProjectID      string                `json:"dialogflow_project_id"`
	DialogflowLandcode       string                `json:"dialogflow_langcode"`
	DialogflowEnabled        bool                  `json:"dialogflow_enabled"`
	SyncUserResponses        bool                  `json:"sync_user_responses"`
	TelegramBotToken         string                `json:"telegramBotToken"`
	TelegramModeratorsChat   int64                 `json:"telegramModeratorsChat"`
	RebootsByUserDisabled    bool                  `json:"reboots_by_user_disabled"`
	AutoRebootDisabled       bool                  `json:"auto_reboot_disabled"`
	HealthCheckStrictMode    bool                  `json:"healthcheck_strict_mode"`
	UserMessageRateTimeoutMs int64                 `json:"user_message_rate_timeout_ms"`
	TelegramNotifyChatID     int64                 `json:"tg_notify_chatid"`
	Tips                     []string              `json:"tips"`
	CoinsWithdrawLabel       string                `json:"coins_withdraw_label"`
	GameVoucherPrefix        string                `json:"game_voucher_prefix"`
}

type pointsInterval struct {
	From  int     `json:"from"`
	To    int     `json:"to"`
	Value float64 `json:"value"`
}

type dbHandler struct {
	Conn       *sql.DB
	UsersTable string
}

type dbConnectionTask struct {
	User       string `json:"user"`
	Pass       string `json:"pass"`
	Host       string `json:"host"`
	DB         string `json:"dbname"`
	Port       string `json:"port"`
	UsersTable string `json:"table"`
}

type userData struct {
	UID      string
	Pubkey   string
	NickName string
	Balance  float64
}

type handlerPair struct {
	endpoint    interface{}
	handler     interface{}
	description string
}
