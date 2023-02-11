package main

import (
	"fmt"
	"strconv"

	utopiago "github.com/Sagleft/utopialib-go"
	"github.com/google/logger"
)

func (app *solution) initUsersOnline() error {
	logger.Info("init users online data..")

	contacts, err := app.Config.UtopiaCfg.GetContacts("")
	if err != nil {
		return err
	}
	logger.Info("found contacts: " + strconv.Itoa(len(contacts)))

	for _, contact := range contacts {
		if isUserOnline(contact) && contact.Nick != serviceAccountName {
			app.markUserOnline(contact.Pubkey)
		}
	}
	return nil
}

func (app *solution) isUserInOnlineData(pubkey string) bool {
	_, isExists := app.UsersOnline[pubkey]
	return isExists
}

func (app *solution) markUserOnline(pubkey string) {
	app.UsersOnline[pubkey] = &onlineData{
		Pubkey: pubkey,
	}
}

func (app *solution) markUserOffline(pubkey string) {
	//if app.isUserInOnlineData(pubkey) {
	delete(app.UsersOnline, pubkey)
	//}
}

func (app *solution) handleWsConnected() {
	logger.Info("ws connection established")
}

func (app *solution) handleWsEvent(event utopiago.WsEvent) {
	handler, isHandlerFound := app.WsHandlers[event.Type]
	if !isHandlerFound {
		return
	}
	handler(event)
}

/*
{
    "data": {
        "nick": "JNox",
        "pk": "954220E969D803D8E19CE5DDD00DE85563AD89C9FC6882CE56C228BA88279C6A",
        "received": "2022-05-28T19:41:30.285Z"
    },
    "type": "newAuthorization"
}
*/
func (app *solution) onNewAuth(event utopiago.WsEvent) {
	// get pubkey
	userPubkey, err := event.GetString("pk")
	if err != nil {
		logger.Error(err)
		return
	}

	nick, err := event.GetString("nick")
	if err != nil {
		logger.Error(err)
		return
	}

	// save user pubkey
	_, err = app.DB.getUserData(userPubkey, filterNickname(nick))
	if err != nil {
		logger.Error(err)
		return
	}

	// approve auth
	_, err = app.Config.UtopiaCfg.AcceptAuthRequest(userPubkey, "")
	if err != nil {
		app.onUtopiaError(fmt.Errorf("failed to accept auth: %w", err))
		return
	}

	logger.Info("user " + userPubkey + " auth accepted")
	for i := 0; i < len(app.Config.WelcomeMessages); i++ {
		err = app.sendMessage(userPubkey, app.Config.WelcomeMessages[i])
		if err != nil {
			app.onUtopiaError(fmt.Errorf("failed to send PM: %w", err))
		}
	}
}

/*
{
    "data": {
        "nick": "",
        "pk": "4FB62131A403EE7D00C0ECAA85D68A6F8C21B717023B45EF8B26F81C03DF1A18",
        "status": "Offline",
        "statusCode": 65536
    },
    "type": "contactStatusNotification"
}
*/
func (app *solution) onContactNotify(event utopiago.WsEvent) {
	userPubkey, err := event.GetString("pk")
	if err != nil {
		logger.Error(err)
		return
	}

	statusCode, err := event.GetFloat("statusCode")
	if err != nil {
		logger.Error(err)
		return
	}

	if isUserOnline(utopiago.ContactData{
		Status: int(statusCode),
	}) {
		logger.Info(userPubkey + " online")
		app.markUserOnline(userPubkey)
	} else {
		logger.Info(userPubkey + " offline")
		app.markUserOffline(userPubkey)
	}

	err = app.handleContact(handleContactTask{
		Pubkey:           userPubkey,
		WithPayment:      false,
		ChannelOnlineMap: make(map[string]utopiago.ChannelContactData),
	})
	if err != nil {
		logger.Error(err)
	}
}
