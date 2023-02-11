package main

import (
	"bytes"
	"log"
	"os/exec"
	"strconv"
	"time"

	tb "github.com/Sagleft/telegobot"
	"github.com/google/logger"
)

func (app *solution) tgMessageFilter(upd *tb.Update) bool {
	if upd.Message == nil {
		return true
	}

	return true
}

func (app *solution) getTgPoller() *tb.MiddlewarePoller {
	poller := &tb.LongPoller{Timeout: 15 * time.Second}
	return tb.NewMiddlewarePoller(poller, app.tgMessageFilter)
}

func (app *solution) tgConnect() error {
	var err error
	app.TelegramBot, err = tb.NewBot(tb.Settings{
		Token:  app.Config.TelegramBotToken,
		Poller: app.getTgPoller(),
	})
	return err
}

func (app *solution) setupHandlers(handlers []handlerPair) {
	if len(handlers) == 0 {
		log.Println("handlers is not set")
		return
	}
	for _, pair := range handlers {
		app.TelegramBot.Handle(pair.endpoint, pair.handler)
	}
}

func (app *solution) runTelegramBot() error {
	app.TelegramHandlers = []handlerPair{
		{"/start", app.handleStart, "узнать свой telegram ID"},
		{"/reboot", app.handleReboot, "перезагрузить весь сервер (попросит подтверждение)"},
		{"/confirmreboot", app.confirmHandleReboot, "подтвердить перезагрузку"},
		{"/restartutopia", app.handleRestartUtopia, "перезагрузить сервис утопии"},
		{"/restartbot", app.handleRestartBot, "перезагрузить сервис бота"},
		{"/contacts", app.getContacts, "получить список онлайна (с учетом канала) файлом"},
		{"/onlinecount", app.getOnlineCount, "узнать число онлайна"},
		{tb.OnText, app.handleTextRequest, ""},
	}
	app.setupHandlers(app.TelegramHandlers)

	go app.TelegramBot.Start()
	return nil
}

func (app *solution) getOnlineCount(m *tb.Message) {
	if !app.checkTelegramAccess(m) {
		return
	}

	contactsData, err := app.getContactsData()
	if err != nil {
		app.returnErrorToSender(m, err)
		return
	}

	msg := "Всего контактов: " + strconv.Itoa(contactsData.Contacts) + "\n"
	msg += "Контактов онлайн: " + strconv.Itoa(contactsData.ContactsOnline) + "\n"
	msg += "Онлайн в канале: " + strconv.Itoa(contactsData.ChannelOnline) + "\n"
	msg += "Контактов онлайн в канале: " + strconv.Itoa(contactsData.ContactsInChannel)

	app.TelegramBot.Send(m.Sender, msg)
}

func (app *solution) getMessages() string {
	msg := ""
	for i := 0; i < len(app.TelegramHandlers); i++ {
		h := app.TelegramHandlers[i]
		if h.description != "" {
			msg += h.endpoint.(string) + " - " + h.description + "\n"
		}
	}
	return msg
}

func (app *solution) handleStart(m *tb.Message) {
	app.TelegramBot.Send(m.Sender, strconv.FormatInt(m.Sender.ID, 10))
}

type getContactsResult struct {
	CSV               string
	Contacts          int
	ContactsOnline    int
	ChannelOnline     int
	ContactsInChannel int
}

func (app *solution) getContactsData() (*getContactsResult, error) {
	contacts, err := app.Config.UtopiaCfg.GetContacts("")
	if err != nil {
		return nil, err
	}

	channelOnline, err := app.getChannelOnline()
	if err != nil {
		return nil, err
	}
	channelOnlineMap := app.getChannelOnlineMap(channelOnline)

	result := getContactsResult{
		Contacts:      len(contacts),
		ChannelOnline: len(channelOnlineMap),
	}
	result.CSV = "nick, pubkey, online, online in channel"
	for _, contact := range contacts {
		result.CSV += "\n" + contact.Nick + ", " + contact.Pubkey

		if app.isUserInOnlineData(contact.Pubkey) {
			result.ContactsOnline++
			result.CSV += ", +"
		} else {
			result.CSV += ", -"
		}

		_, isOnlineInChannel := channelOnlineMap[contact.Nick]
		if isOnlineInChannel {
			result.CSV += ", +"
			result.ContactsInChannel++
		} else {
			result.CSV += ", -"
		}
	}

	return &result, nil
}

func (app *solution) returnErrorToSender(m *tb.Message, err error) {
	_, err2 := app.TelegramBot.Send(m.Sender, "ERROR: "+err.Error())
	if err2 != nil {
		logger.Error(err2)
	}
}

func (app *solution) getContacts(m *tb.Message) {
	contactsData, err := app.getContactsData()
	if err != nil {
		app.returnErrorToSender(m, err)
		return
	}

	dataBytes := []byte(contactsData.CSV)
	reader := bytes.NewReader(dataBytes)

	_, err = app.TelegramBot.Send(m.Sender, &tb.Document{
		File:     tb.FromReader(reader),
		MIME:     "text/plain",
		FileName: "contacts.txt",
	})
	if err != nil {
		app.returnErrorToSender(m, err)
	}
}

func (app *solution) checkTelegramAccess(m *tb.Message) bool {
	if !app.isUserTelegramModerator(m.Sender.ID) {
		_, err := app.TelegramBot.Send(m.Sender, &tb.Audio{
			File:    tb.FromDisk("access_denied.mp3"),
			Caption: "🔒",
		})
		if err != nil {
			logger.Error(err)
		}
		return false
	}

	return true
}

func (app *solution) checkRebootsFeatureDisabled(m *tb.Message) bool {
	if app.Config.RebootsByUserDisabled {
		_, err := app.TelegramBot.Send(m.Sender, "фича отключена")
		if err != nil {
			logger.Error(err)
			return true
		}
	}
	return app.Config.RebootsByUserDisabled
}

func (app *solution) handleReboot(m *tb.Message) {
	if !app.checkTelegramAccess(m) {
		return
	}

	if app.checkRebootsFeatureDisabled(m) {
		return
	}

	_, err := app.TelegramBot.Send(
		m.Sender,
		"А может не надо? Может лучше через /restartbot ? или /restartutopia ?\n\nНу а если всё совсем плохо...\n/confirmreboot",
	)
	if err != nil {
		logger.Error(err)
		return
	}
}

func (app *solution) confirmHandleReboot(m *tb.Message) {
	if !app.checkTelegramAccess(m) {
		return
	}

	if app.checkRebootsFeatureDisabled(m) {
		return
	}

	_, err := app.TelegramBot.Send(m.Sender, "Сервер будет перезагружен через 3 секунды..")
	if err != nil {
		logger.Error(err)
		return
	}

	time.Sleep(time.Second * 3)
	r := exec.Command("reboot")
	err = r.Run()
	if err != nil {
		logger.Error(err)
		app.TelegramBot.Send(m.Sender, "Не удалось заребутить: "+err.Error())
	}
}

func (app *solution) handleRestartUtopia(m *tb.Message) {
	if !app.checkTelegramAccess(m) {
		return
	}

	if app.checkRebootsFeatureDisabled(m) {
		return
	}

	_, err := app.TelegramBot.Send(m.Sender, "U и бот будут перезагружены через 3 секунды..")
	if err != nil {
		logger.Error(err)
		return
	}

	err = doUtopiaReboot()
	if err != nil {
		app.TelegramBot.Send(m.Sender, "Не удалось перезапустить: "+err.Error())
	}
}

func (app *solution) handleRestartBot(m *tb.Message) {
	if !app.checkTelegramAccess(m) {
		return
	}

	if app.checkRebootsFeatureDisabled(m) {
		return
	}

	_, err := app.TelegramBot.Send(m.Sender, "Бот будет перезагружен через 3 секунды..")
	if err != nil {
		logger.Error(err)
		return
	}

	time.Sleep(time.Second * 3)
	r := exec.Command("/usr/bin/systemctl", "restart", "bankbot")
	err = r.Run()
	if err != nil {
		logger.Error(err)
		app.TelegramBot.Send(m.Sender, "Не удалось перезапустить: "+err.Error())
	}
}

func (app *solution) handleTextRequest(m *tb.Message) {
	if !app.checkTelegramAccess(m) {
		return
	}

	messages, err := app.handleModeratorRequest(m.Text, true, m.Sender.ID)
	if err != nil {
		_, tgErr := app.TelegramBot.Send(m.Sender, "ERROR: "+err.Error())
		if tgErr != nil {
			logger.Error(tgErr)

			app.TelegramBot.Send(m.Sender, "ERROR: "+tgErr.Error())
		}
		return
	}

	for i := 0; i < len(messages); i++ {
		_, tgErr := app.TelegramBot.Send(m.Sender, messages[i])
		if tgErr != nil {
			logger.Error(tgErr)

			app.TelegramBot.Send(m.Sender, "ERROR: "+tgErr.Error())
		}
	}
}

type sendNotifyTask struct {
	Nickname string
	Amount   float64
}

func (app *solution) sendWithdrawNotify(task sendNotifyTask) error {
	if isOver, _ := app.WithdrawNotifyRateLimiter.Try(); !isOver {
		return nil
	}

	if task.Nickname == "" {
		task.Nickname = "Anonymous"
	}

	if task.Amount < app.Config.MinWithdraw {
		// не оповещаем о выводах меньше минимального
		return nil
	}

	// send notify to telegram
	if app.Config.TelegramNotifyChatID != 0 {

		msg := "🎩  *Игрок " + task.Nickname + "* вывел\n" +
			getRandomTgEmoji() + "  *" + strconv.FormatFloat(task.Amount, 'f', 0, 64) + "* " + app.Config.CoinsWithdrawLabel

		if _, err := app.TelegramBot.Send(tb.ChatID(app.Config.TelegramNotifyChatID), msg, tb.ModeMarkdown); err != nil {
			return err
		}

	}

	// TODO: send to U?

	return nil
}
