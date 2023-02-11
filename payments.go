package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/google/logger"
)

func (app *solution) viewUserBalance(userPubkey string) (string, error) {
	if userPubkey == "" {
		return "проверь правильность запроса, я не смог найти в нем публичный ключ", nil
	}

	if len(userPubkey) != 64 {
		return "Неверная длина публичного ключа юзера", nil
	}

	if app.DB == nil {
		return "Хьюстон! У нас проблемы! Отсутствует подключение к базе данных", nil
	}

	uData, err := app.DB.getUserDBData(userPubkey)
	if err != nil {
		return "", err
	}

	if uData == nil {
		return "Пользователь не найден", nil
	}

	return "На балансе юзера " + formatFloat(uData.Balance) + " б", nil
}

func (app *solution) resetUserPoints(userPubkey string) (string, error) {
	uData, err := app.DB.getUserDBData(userPubkey)
	if err != nil {
		return "", err
	}

	err = app.DB.resetUserPoints(userPubkey)
	if err != nil {
		return "", err
	}
	return "Сброс баллов юзера №" + uData.UID + " выполнен", nil
}

func (app *solution) decreaseUserPoints(userPubkey, pointsRaw string) (string, error) {
	points, err := strconv.ParseFloat(pointsRaw, 64)
	if err != nil {
		return "Я не смог разобрать число поинтов для вычета. Формат команды:\n\n" +
			"вычет ключ количество", nil
	}

	uData, err := app.DB.getUserDBData(userPubkey)
	if err != nil {
		return "", err
	}

	newBalance := uData.Balance - points
	if newBalance < 0 {
		newBalance = 0
	}

	err = app.DB.setUserPoints(userPubkey, newBalance)
	if err != nil {
		return "", err
	}

	if points > 0 {
		if err = app.sendWithdrawNotify(sendNotifyTask{
			Nickname: uData.NickName,
			Amount:   points,
		}); err != nil {
			return "", fmt.Errorf("не удалось отправить оповещение: %w", err)
		}
	}

	msg := "У юзера было " + formatFloat(uData.Balance) + ", вычли " + formatFloat(points) +
		", осталось " + formatFloat(newBalance)
	logger.Info(msg)
	return msg, nil
}

func (app *solution) isVoucherCanBeActivated(userPubkey string) bool {
	timeoutData, isExists := app.VouchersCooldown[userPubkey]
	if !isExists {
		app.VouchersCooldown[userPubkey] = time.Now()
		return true
	}

	return time.Since(timeoutData) > gameVoucherActivateTimeout
}
