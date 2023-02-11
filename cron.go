package main

import (
	"errors"
	"fmt"
	"os/exec"
	"time"

	tb "github.com/Sagleft/telegobot"
	utopiago "github.com/Sagleft/utopialib-go"
	"github.com/google/logger"
	simplecron "github.com/sagleft/simple-cron"
)

func (app *solution) setupCrons() error {
	logger.Info("setup cron..")

	return checkErrors(
		app.setupContactStatusesCron,
		app.setupHealthckechCron,
	)
}

func (app *solution) setupContactStatusesCron() error {
	app.HandleContactsCron = simplecron.NewCronHandler(
		app.handleContacts, // callback
		time.Duration(app.getContactsCronTimeoutSeconds())*time.Second, // timeout
	)
	go app.HandleContactsCron.Run()
	return nil
}

func (app *solution) setupHealthckechCron() error {
	cron := simplecron.NewCronHandler(
		app.doHealthCheck,  // callback
		healthCheckTimeout, // timeout
	)
	go cron.Run()
	return nil
}

func (app *solution) doHealthCheck() {
	// check connection
	if !app.Config.UtopiaCfg.CheckClientConnection() {
		err := app.utopiaConnect()
		if err != nil {
			logger.Error(err)
			doUtopiaReboot()
			return
		}
	}

	// check data
	contactsData, err := app.getContactsData()
	if err != nil {
		logger.Error(err)
		doUtopiaReboot()
		return
	}
	if app.Config.HealthCheckStrictMode {
		if contactsData.Contacts == 0 {
			logger.Error("contacts not found")
			doUtopiaReboot()
			return
		}
	}
}

func doUtopiaReboot() error {
	if autoRebootDisabled {
		logger.Info("skip utopia reboot: disabled by config")
		return nil
	}

	logger.Info("reboot utopia service..")

	r := exec.Command("/usr/bin/systemctl", "restart", "startopia")
	err := r.Run()
	if err != nil {
		logger.Error(err)
	}
	return err
}

func doBotReboot() error {
	if autoRebootDisabled {
		logger.Info("bot autoreboot disabled by config")
		return nil
	}

	logger.Info("reboot bot service..")

	r := exec.Command("/usr/bin/systemctl", "restart", "bankbot")
	err := r.Run()
	if err != nil {
		logger.Error(err)
	}
	return err
}

func reconnect(connectionName string, connMethod func() error) error {
	for {
		for i := 1; i <= 5; i++ {
			logger.Info("connect to Utopia Network..")
			err := connMethod()
			if err == nil {
				logger.Info("connected")
				return nil
			}

			logger.Warning("connection failed")
			time.Sleep(time.Second * 12)
		}

		if err := doUtopiaReboot(); err != nil {
			logger.Error(err)
		}

		logger.Warning("retry after 40s..")
		time.Sleep(time.Second * 40)
	}

	//return errors.New("failed to connect to " + connectionName)
}

func (app *solution) utopiaConnect() error {
	err := reconnect("utopia", func() error {
		if !app.Config.UtopiaCfg.CheckClientConnection() {
			return errors.New("failed to connect to " + app.Config.UtopiaCfg.Host)
		}

		return app.setupUtopiaWs()
	})
	if err != nil {
		return err
	}

	// setup logger
	app.Config.UtopiaCfg.SetLogsCallback(app.onDebugLog)
	return nil
}

func (app *solution) onDebugLog(logMessage string) {
	logger.Info(logMessage)
}

func (app *solution) setupUtopiaWs() error {
	print("setup websocket connection..")

	err := app.Config.UtopiaCfg.SetWebSocketState(utopiago.SetWsStateTask{
		Enabled:       true,
		Port:          app.Config.UtopiaCfg.WsPort,
		EnableSSL:     false,
		Notifications: "contact",
	})
	if err != nil {
		return err
	}

	return app.Config.UtopiaCfg.WsSubscribe(utopiago.WsSubscribeTask{
		OnConnected: app.handleWsConnected,
		Callback:    app.handleWsEvent,
		ErrCallback: onWsError,
	})
}

func (app *solution) getContactsCronTimeoutSeconds() int {
	return app.Config.ContactsCronPerMinute * 60
}

func (app *solution) lockContactsCheck() {
	app.IsContactsCheckInProgress = true
}

func (app *solution) unlockContactsCheck() {
	app.IsContactsCheckInProgress = false
}

func (app *solution) handleContacts() {
	if app.IsContactsCheckInProgress {
		return
	}
	app.lockContactsCheck()
	defer app.unlockContactsCheck()

	contacts, err := app.Config.UtopiaCfg.GetContacts("")
	if err != nil {
		app.onUtopiaError(err)

		if len(app.ContactsOnlineCache) == 0 {
			return
		}
		contacts = app.ContactsOnlineCache
	} else {
		app.ContactsOnlineCache = contacts
	}

	channelOnline, err := app.getChannelOnline()
	if err != nil {
		app.onUtopiaError(err)

		// use cache when available
		if len(app.ChannelOnlineCache) == 0 {
			return
		}
		channelOnline = app.ChannelOnlineCache
	} else {
		app.ChannelOnlineCache = channelOnline
	}

	channelOnlineMap := app.getChannelOnlineMap(channelOnline)
	usersOnline, err := app.getUsersOnlineCount(contacts, channelOnlineMap)
	if err != nil {
		logger.Error(err)
		return
	}

	for pubkey := range app.UsersOnline {
		err := app.handleContact(handleContactTask{
			Pubkey:           pubkey,
			WithPayment:      true,
			ChannelOnlineMap: channelOnlineMap,
			UsersOnlineCount: usersOnline,
		})
		if err != nil {
			app.onUtopiaError(err)
			break
		}
	}
}

func (app *solution) onUtopiaError(err error) {
	logger.Error(err)

	logger.Info("check is connection broken..")
	if utopiago.CheckErrorConnBroken(err) {
		logger.Error("connection is broken. reboot..")

		if err := doUtopiaReboot(); err != nil {
			logger.Error(fmt.Errorf("failed to reboot Utopia: %w", err))
			return
		}

		logger.Info("wait " + waitAfterUtopiaReboot.String() + " to reboot U..")
		time.Sleep(waitAfterUtopiaReboot)

		if err := app.utopiaConnect(); err != nil {
			logger.Error(fmt.Errorf("failed to reconnect Utopia: %w", err))
			return
		}
		return
	}

	app.notifyModeratorsAboutError(err)
	logger.Error(err)
}

func (app *solution) notifyModeratorsAboutError(err error) {
	if err == nil {
		return
	}

	if app.Config.TelegramModeratorsChat != 0 {
		msg := "🤖 Ошибка соединения или запроса: " + err.Error()
		if _, tgErr := app.TelegramBot.Send(tb.ChatID(app.Config.TelegramModeratorsChat), msg); tgErr != nil {
			logger.Error(tgErr)
		}
	}
}

func isUserOnline(contact utopiago.ContactData) bool {
	return contact.IsOnline() || contact.IsAway() || contact.IsDoNotDisturb()
}

// с учетом онлайна в чате
func (app *solution) getUsersOnlineCount(
	contacts []utopiago.ContactData,
	channelOnlineMap map[string]utopiago.ChannelContactData,
) (int, error) {
	var usersOnline int = 0
	for _, contact := range contacts {
		_, isUserOnlineInChannel := channelOnlineMap[contact.Pubkey]
		if isUserOnlineInChannel {
			usersOnline++
		}
	}
	return usersOnline, nil
}

type handleContactTask struct {
	Pubkey           string
	WithPayment      bool
	ChannelOnlineMap map[string]utopiago.ChannelContactData
	UsersOnlineCount int // used with WithPayment param
}

func (app *solution) handleContact(task handleContactTask) error {
	if app.isUserModerator(task.Pubkey) {
		return nil // ignore on moderator contact
	}

	contact, err := app.Config.UtopiaCfg.GetContact(task.Pubkey)
	if err != nil {
		return err
	}

	if contact.Nick == serviceAccountName {
		return nil
	}

	_, isOnlineInChannel := task.ChannelOnlineMap[contact.Nick]
	if !isOnlineInChannel {
		return nil // user not online in channel
	}

	//if app.isUserInOnlineData(task.Pubkey) {
	if task.WithPayment {
		points := app.getPointsByPeriod(task.UsersOnlineCount)
		//logger.Info("добавление " + formatFloat(points) + " пользователю " + task.Pubkey)
		err := app.DB.addUserPoints(points, task.Pubkey)
		if err != nil {
			return err
		}
	}
	return nil
}
