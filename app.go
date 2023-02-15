package main

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	utopiago "github.com/Sagleft/utopialib-go"
	"github.com/beefsack/go-rate"
	"github.com/common-nighthawk/go-figure"
	"github.com/google/logger"
)

func newSolution() solution {
	return solution{
		WithdrawNotifyRateLimiter: rate.New(1, limitWithdrawNotifyTimeout),
		UsersOnline:               map[string]*onlineData{},
		VouchersCooldown:          make(map[string]time.Time),
	}
}

type wsHandler func(event utopiago.WsEvent)

func (app *solution) setupWsHandlers() error {
	logger.Info("setup utopia ws handlers..")

	app.WsHandlers = map[string]wsHandler{
		"newAuthorization":          app.onNewAuth,
		"contactStatusNotification": app.onContactNotify,
		"newInstantMessage":         app.onUserMessage,
		"newOutgoingInstantMessage": func(event utopiago.WsEvent) {}, // placeholder
	}
	return nil
}

func onWsError(err error) {
	logger.Error(err.Error())
}

func (app *solution) runInBackground() {
	forever := make(chan struct{})
	// run in backround
	<-forever
}

func main() {
	figure.NewColorFigure(" talk2earn $$$", "", "green", true).
		Scroll(3*1000, 200, "left")

	app := newSolution()

	initLogger()
	defer logsHandler.Close()
	defer logsFile.Close()

	err := checkErrors(
		app.parseConfig,
		app.initVouchers,
		app.setupModerators,
		app.tgConnect,
		app.runTelegramBot,
		app.utopiaConnect,
		app.parseArgs,
		app.tryEnterChannel,
		app.setupCrons,
		app.initUsersOnline,
	)
	if err != nil {
		logger.Error(err)
		return
	}

	printSuccess("bot initiated")
	logger.Info("bot initiated")
	app.runInBackground()
}

func (app *solution) initVouchers() error {
	gameVoucherLength = len(app.genGameVoucher())
	return nil
}

func (app *solution) setupModerators() error {
	logger.Info("setup moderators..")
	app.UtopiaModerators = make(map[string]struct{})
	app.TelegramModerators = make(map[int64]struct{})
	for _, pubkey := range app.Config.ModeratorPubkeys {
		app.UtopiaModerators[pubkey] = struct{}{}
	}
	for _, tid := range app.Config.ModeratorTelegramIDs {
		app.TelegramModerators[tid] = struct{}{}
	}
	return nil
}

func (app *solution) isUserModerator(pubkey string) bool {
	_, isModerator := app.UtopiaModerators[pubkey]
	return isModerator
}

func (app *solution) isUserTelegramModerator(telegramID int64) bool {
	_, isModerator := app.TelegramModerators[telegramID]
	return isModerator
}

func (app *solution) tryEnterChannel() error {
	logger.Info("enter into utopia channel..")

	_, err := app.Config.UtopiaCfg.JoinChannel(app.Config.ChannelID)
	app.onUtopiaError(err)
	return nil
}

func (app *solution) parseArgs() error {
	logger.Info("parse args..")

	for _, arg := range os.Args[1:] {
		if arg == "notify" {
			if err := app.sendNotifyToAllUsers(); err != nil {
				return err
			}
			app.exit()
		}
		if arg == "testOnline" {
			if err := app.testUserOnline(); err != nil {
				return err
			}
			app.exit()
		}
		if arg == "updateNicks" {
			if err := app.updateNicknames(); err != nil {
				return err
			}
			app.exit()
		}
	}
	return nil
}

func (app *solution) exit() {
	os.Exit(1)
}

func (app *solution) updateNicknames() error {
	task := updateNicknameTask{}

	contacts, err := app.Config.UtopiaCfg.GetContacts("")
	if err != nil {
		return err
	}

	for _, contact := range contacts {
		task[contact.Pubkey] = filterNickname(contact.Nick)
	}

	return app.DB.updateNicknames(task)
}

func (app *solution) testUserOnline() error {
	contact, err := app.Config.UtopiaCfg.GetContact(testUserOnlinePubkey)
	if err != nil {
		return err
	}

	contactDataBytes, err := json.MarshalIndent(contact, "", "\t")
	if err != nil {
		return err
	}

	logger.Info(string(contactDataBytes))
	return nil
}

func (app *solution) sendNotifyToAllUsers() error {
	logger.Info("send notify to all users..")

	contacts, err := app.Config.UtopiaCfg.GetContacts("")
	if err != nil {
		return err
	}

	for _, contact := range contacts {
		msg := "Внимание!\n" +
			"Бот приостановлен! Для продолжения работы бота вам необходимо нажать кнопку присоединиться. После присоедиенения бот автоматически продолжит свою работу."

		err := app.sendMessage(contact.Pubkey, msg)
		if err != nil {
			if strings.Contains(err.Error(), "Rate limit exceeded") {
				err := app.sendMessage(contact.Pubkey, msg)
				if err != nil {
					return err
				}
			} else {
				app.onUtopiaError(err)
			}
		}

		msg = "C625377297CFBEAA6417FCB76DAAFC3D"
		err = app.sendMessage(contact.Pubkey, msg)
		if err != nil {
			if strings.Contains(err.Error(), "Rate limit exceeded") {
				err := app.sendMessage(contact.Pubkey, msg)
				if err != nil {
					return err
				}
			}
		}

		logger.Info("send notify to user " + contact.Nick)
		time.Sleep(time.Second * 2)
	}
	return nil
}
