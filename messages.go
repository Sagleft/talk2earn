package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	dialogflow "cloud.google.com/go/dialogflow/apiv2"
	utopiago "github.com/Sagleft/utopialib-go"
	"github.com/google/logger"
	dialogflowpb "google.golang.org/genproto/googleapis/cloud/dialogflow/v2"
)

/*
{
    "data": {
        "dateTime": "2022-05-28T21:47:34.110Z",
        "file": null,
        "id": 309,
        "isIncoming": true,
        "messageType": 1,
        "metaData": null,
        "nick": "JNox",
        "pk": "954220E969D803D8E19CE5DDD00DE85563AD89C9FC6882CE56C228BA88279C6A",
        "readDateTime": "2022-05-28T21:47:33.325Z",
        "receivedDateTime": "2022-05-28T21:47:34.110Z",
        "text": "test"
    },
    "type": "newInstantMessage"
}
*/
func (app *solution) onUserMessage(event utopiago.WsEvent) {
	isMessageIncoming, err := event.GetBool("isIncoming")
	if err != nil {
		logger.Error(err)
		return
	}

	if !isMessageIncoming {
		return
	}

	messageText, err := event.GetString("text")
	if err != nil {
		logger.Error(err)
		return
	}

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

	// если это игровой ваучер, который прислан без команд
	if len(messageText) == gameVoucherLength || strings.Contains(messageText, app.Config.GameVoucherPrefix) {

		if !app.isVoucherCanBeActivated(userPubkey) {
			if err := app.sendMessage(userPubkey, "ваучер уже был активирован или не существует"); err != nil {
				app.onUtopiaError(err)
				return
			}
			return
		}

		voucherAmount, err := app.activateGameVoucher(userPubkey, messageText)
		if err != nil {
			logger.Error(err)
			msg := "Произошла ошибка при активации ваучера.\n" +
				"Можешь связаться с менеджером, сообщив дату и время ошибки"
			if err := app.sendMessage(userPubkey, msg); err != nil {
				app.onUtopiaError(err)
				return
			}
			return
		}

		if voucherAmount == 0 {
			if err := app.sendMessage(userPubkey, "ваучер уже был активирован или не существует"); err != nil {
				app.onUtopiaError(err)
				return
			}
			return
		}

		msg := fmt.Sprintf("OK! Ваучер был активирован\nНачислено +%v баллов", voucherAmount)
		if err := app.sendMessage(userPubkey, msg); err != nil {
			app.onUtopiaError(err)
			return
		}
		return
	}

	if app.isUserModerator(userPubkey) {
		// moderator request
		messages, err := app.handleModeratorRequest(messageText, false, 0)
		if err != nil {
			logger.Error(err)
		}

		for i := 0; i < len(messages); i++ {
			err = app.sendMessageWithoutLock(userPubkey, messages[i])
			if err != nil {
				app.onUtopiaError(err)
			}
		}

		return
	}

	userData, err := app.DB.getUserData(userPubkey, filterNickname(nick))
	if err != nil {
		logger.Error(err)
		if err := app.sendMessage(userPubkey, "ошибка обработки запроса"); err != nil {
			app.onUtopiaError(err)
			return
		}
		return
	}

	if len(messageText) < 3 {
		err = app.sendMessage(userPubkey, "Сообщение слишком короткое")
		if err != nil {
			app.onUtopiaError(err)
			return
		}
		return
	}

	// КОМАНДЫ ЮЗВЕРЯ
	messageText = strings.TrimSpace(messageText)
	messageText = strings.ToLower(messageText)
	replyMessage := ""
	switch messageText {
	default:
		replyMessage, err = app.handleUnknownUserMessage(messageText)
		if err != nil {
			logger.Error(err)
			if err := app.sendMessage(userPubkey, "не удалось обработать запрос"); err != nil {
				app.onUtopiaError(err)
				return
			}
			return
		}
	case comandBalance:
		replyMessage = app.getUserBalance(userData)
	case comandBalance2:
		replyMessage = app.getUserBalance(userData)
	case comandManager:
		replyMessage = "Чтобы вывести баллы, можно писать: " + app.Config.RequestsModeratorPubkey + "\n" +
			"Или в телеграме - " + app.Config.ModeratorTelegram
	}

	err = app.sendMessage(userPubkey, replyMessage)
	if err != nil {
		app.onUtopiaError(err)
	}
}

// returns voucher amount
func (app *solution) activateGameVoucher(userPubkey, voucherCode string) (float64, error) {
	amount, err := app.DB.getGameVoucherAmount(voucherCode)
	if err != nil {
		return 0, err
	}
	if amount == 0 {
		return 0, nil
	}

	if err := app.DB.addUserPoints(amount, userPubkey); err != nil {
		return 0, err
	}

	return amount, app.DB.deleteGameVoucher(voucherCode)
}

func (app *solution) getUserBalance(userData *userData) string {
	return ""

	replyMessage := "Текущий баланс: " + formatFloat(userData.Balance) + " баллов.\n" +
		"Минимальный вывод: " + formatFloat(app.Config.MinWithdraw) + "."

	if userData.Balance >= app.Config.MinWithdraw {
		replyMessage += "\n\nДля вывода средств необходимо связаться с " + app.Config.RequestsModeratorPubkey + "\n\n" +
			"Или в телеграм: " + app.Config.ModeratorTelegram
	}

	replyMessage += "\n\n[forefinger] " + getRandomTip()
	return replyMessage
}

func (app *solution) handleUnknownUserMessage(messageText string) (string, error) {
	var err error
	var replyMessage string

	if app.Config.DialogflowEnabled {
		replyMessage, err = app.handleDialogFlowMessage(messageText)
	} else {
		replyMessage = app.Config.InvalidMessage
	}

	return replyMessage, err
}

func (app *solution) handleDialogFlowMessage(messageText string) (string, error) {
	return DetectIntentText(
		app.Config.DialogflowProjectID,
		dialogFlowSessionID,
		messageText,
		app.Config.DialogflowLandcode,
	)
}

func DetectIntentText(projectID, sessionID, text, languageCode string) (string, error) {
	ctx := context.Background()

	sessionClient, err := dialogflow.NewSessionsClient(ctx)
	if err != nil {
		return "", err
	}
	defer sessionClient.Close()

	if projectID == "" || sessionID == "" {
		return "", fmt.Errorf("received empty project (%s) or session (%s)", projectID, sessionID)
	}

	sessionPath := fmt.Sprintf("projects/%s/agent/sessions/%s", projectID, sessionID)
	textInput := dialogflowpb.TextInput{Text: text, LanguageCode: languageCode}
	queryTextInput := dialogflowpb.QueryInput_Text{Text: &textInput}
	queryInput := dialogflowpb.QueryInput{Input: &queryTextInput}
	request := dialogflowpb.DetectIntentRequest{Session: sessionPath, QueryInput: &queryInput}

	response, err := sessionClient.DetectIntent(ctx, &request)
	if err != nil {
		return "", err
	}

	queryResult := response.GetQueryResult()
	fulfillmentText := queryResult.GetFulfillmentText()
	return fulfillmentText, nil
}

func (app *solution) sendMessage(pubkey string, text string) error {
	if app.Config.SyncUserResponses {
		return app.sendMessageWithLock(pubkey, text)
	}

	return app.sendMessageWithoutLock(pubkey, text)
}

func (app *solution) sendMessageWithLock(pubkey string, text string) error {
	// sync messages
	app.MessageHandler.Lock()
	defer app.MessageHandler.Unlock()

	// limit rate
	app.MessageHandler.RateLimiter.Wait()

	// send message
	_, err := app.Config.UtopiaCfg.SendInstantMessage(pubkey, text)
	return err
}

func (app *solution) sendMessageWithoutLock(pubkey string, text string) error {
	// limit rate
	app.MessageHandler.RateLimiter.Wait()

	_, err := app.Config.UtopiaCfg.SendInstantMessage(pubkey, text)
	return err
}

// КОМАНДЫ МОДЕРАТОРА
func (app *solution) handleModeratorRequest(
	messageText string, fromTelegram bool, telegramUserID int64,
) ([]string, error) {
	if messageText == "" {
		return []string{"пустое сообщение"}, nil
	}

	messageText = strings.TrimSpace(messageText)
	msgParts := strings.Split(messageText, " ")

	if len(msgParts) == 0 {
		return []string{"Запрос должен содержать 2 части через пробел:\n\n"}, nil
	}

	command := strings.ToLower(msgParts[0])
	switch command {
	default:
		msgs := []string{"Я не знаю команды `" + command + "`\n\n"}
		if fromTelegram {
			msgs = append(msgs, app.getMessages())
		}
		return msgs, nil
	case "логи":
		if len(msgParts) < 2 {
			return []string{"Запрос должен содержать 2 части через пробел:\n\nлоги <публичный ключ>"}, nil
		}

		return []string{}, app.getLogsByUser(msgParts[1], telegramUserID)
	case "сброс":
		if len(msgParts) < 2 {
			return []string{"Запрос должен содержать 2 части через пробел:\n\nсброс <публичный ключ>"}, nil
		}
		r, err := app.resetUserPoints(msgParts[1])
		if err != nil {
			return []string{}, err
		}
		return []string{r}, nil
	case "баланс":
		if len(msgParts) < 2 {
			return []string{"Запрос должен содержать 2 части через пробел:\n\n"}, nil
		}
		r, err := app.viewUserBalance(msgParts[1])
		if err != nil {
			return []string{}, err
		}
		return []string{r}, nil
	case "вычет":
		if len(msgParts) < 2 {
			return []string{"Запрос должен содержать 2 части через пробел:\n\n" +
				"команда данные"}, nil
		}
		pointsRaw := ""
		if len(msgParts) >= 3 {
			pointsRaw = msgParts[2]
		}
		r, err := app.decreaseUserPoints(msgParts[1], pointsRaw)
		if err != nil {
			return []string{}, err
		}
		return []string{r}, nil
	case "онлайн":
		return app.handleUsersOnlineRequest(fromTelegram)

	case "ваучер":
		if len(msgParts) < 2 {
			return []string{"Запрос должен содержать 2 части через пробел:\n\n" +
				"ваучер сумма\n\nНапример:\n\n" +
				"ваучер 50"}, nil
		}
		return app.handleCreateVoucherRequest(msgParts[1])

	case "погасить":
		if len(msgParts) < 2 {
			return []string{"Запрос должен содержать 2 части через пробел:\n\n" +
				"погасить <код>\n\nНапример:\n\n" +
				"погасить " + app.genGameVoucher()}, nil
		}
		return app.handleVoucherDelete(msgParts[1])
	}
}

func (app *solution) handleVoucherDelete(voucherCode string) ([]string, error) {
	if err := app.DB.deleteGameVoucher(voucherCode); err != nil {
		return nil, err
	}

	return []string{"OK! ваучер был удален"}, nil
}

func (app *solution) handleCreateVoucherRequest(amountRaw string) ([]string, error) {
	amount, err := strconv.ParseFloat(amountRaw, 64)
	if err != nil {
		return nil, fmt.Errorf("parse amount: %w", err)
	}

	if amount <= 0 {
		return nil, errors.New("invalid voucher amount")
	}

	if amount > maxGameVoucherAmount {
		return nil, fmt.Errorf("max voucher amount is %v", maxGameVoucherAmount)
	}

	voucher := app.genGameVoucher()
	if err := app.DB.saveGameVoucher(voucher, amount); err != nil {
		return nil, err
	}

	return []string{
		"Ваучер успешно создан:\n\n" + voucher +
			"\n\nСумма: " + strconv.FormatFloat(amount, 'f', 4, 64),
	}, nil
}

func (app *solution) handleUsersOnlineRequest(fromTelegram bool) ([]string, error) {
	return app.getUsersOnline(fromTelegram)
}

func (app *solution) getChannelOnline() ([]utopiago.ChannelContactData, error) {
	contacts, err := app.Config.UtopiaCfg.GetChannelContacts(app.Config.ChannelID)
	if err != nil {
		return nil, err
	}

	if app.Config.HealthCheckStrictMode && len(contacts) == 0 {
		return nil, doBotReboot()
	}

	return contacts, nil
}

// returns map[nick]data, error
func (app *solution) getChannelOnlineMap(onlineData []utopiago.ChannelContactData) map[string]utopiago.ChannelContactData {
	result := map[string]utopiago.ChannelContactData{}
	for _, contact := range onlineData {
		result[contact.Nick] = contact
	}
	return result
}

func (app *solution) getUsersOnline(fromTelegram bool) ([]string, error) {
	contacts, err := app.Config.UtopiaCfg.GetContacts("")
	if err != nil {
		return []string{}, err
	}

	channelOnline, err := app.getChannelOnline()
	if err != nil {
		return []string{}, err
	}
	channelOnlineMap := app.getChannelOnlineMap(channelOnline)

	var msgParts []string = make([]string, 0)
	var msgPart string
	var usersOnline int = 0
	if len(contacts) > 0 {
		for i := 0; i < len(contacts); i++ {
			contact := contacts[i]
			//isUserOnline
			if app.isUserInOnlineData(contact.Pubkey) && contact.Nick != serviceAccountName {
				var onlineTag string
				if app.isUserModerator(contact.Pubkey) {
					if fromTelegram {
						onlineTag = "😎"
					} else {
						onlineTag = "[lieutenant]"
					}

				} else {
					_, isUserInChannel := channelOnlineMap[contact.Nick]
					if isUserInChannel {
						if fromTelegram {
							onlineTag = "🟩"
						} else {
							onlineTag = "[military]"
						}

					} else {
						if fromTelegram {
							onlineTag = "🟥"
						} else {
							onlineTag = "[hide]"
						}

					}
				}

				msgPart += onlineTag + " " + contact.Nick + ": " + contact.Pubkey + "\n"
				usersOnline++

				if len(msgPart) > 1024 {
					msgParts = append(msgParts, msgPart)
					msgPart = ""
				}

			}
		}

		if msgPart != "" {
			msgParts = append(msgParts, msgPart)
		}

		if len(msgParts) > 0 {
			msgParts[0] = "Пользователи онлайн: " + strconv.Itoa(usersOnline) + "\n" + msgParts[0]
		}
	}

	if usersOnline == 0 {
		return []string{"Никого нет онлайн"}, nil
	}

	return msgParts, nil
}
