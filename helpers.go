package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	swissknife "github.com/Sagleft/swiss-knife"
	"github.com/beefsack/go-rate"
	"github.com/fatih/color"
	"github.com/google/logger"
)

type errorFunc func() error

func checkErrors(errChecks ...errorFunc) error {
	for _, errFunc := range errChecks {
		err := errFunc()
		if err != nil {
			return err
		}
	}
	return nil
}

func (app *solution) parseConfig() error {
	logger.Info("parse config..")

	// parse config file
	if _, err := os.Stat(configJSONPath); os.IsNotExist(err) {
		return errors.New("failed to find config file")
	}

	jsonBytes, err := ioutil.ReadFile(configJSONPath)
	if err != nil {
		return err
	}
	err = json.Unmarshal(jsonBytes, app.Config)
	if err != nil {
		return err
	}

	app.MessageHandler = messagesHandler{
		Client: &app.Config.UtopiaCfg,
		RateLimiter: rate.New(
			limitMaxUserResponsesPerSecond,
			time.Duration(app.Config.UserMessageRateTimeoutMs)*time.Hour,
		),
	}

	autoRebootDisabled = app.Config.AutoRebootDisabled
	tips = app.Config.Tips
	return nil
}

func wrapPrintedMessage(info string) string {
	return "[ " + info + " ]"
}

func printSuccess(info string) {
	color.Green(wrapPrintedMessage(info))
}

func (app *solution) getPointsByPeriod(usersOnline int) float64 {
	var pointsPer24h float64 = 0

	if app.Config.UseIntervals {
		// find users online value from intervals
		// value = points by 1h
		var pointsBy1h float64 = 0
		for i := 0; i < len(app.Config.Intervals); i++ {
			interval := app.Config.Intervals[i]
			if usersOnline >= interval.From && usersOnline <= interval.To {
				pointsBy1h = interval.Value
			}
		}
		if pointsBy1h == 0 {
			logger.Error("interval not found to get points per 24h")
		} else {
			pointsPer24h = pointsBy1h * 24
		}
	} else {
		pointsPer24h = app.Config.PointsPer24h
	}

	return pointsPer24h / (24 * 60 * 60 / float64(app.getContactsCronTimeoutSeconds()))
}

func formatFloat(val float64) string {
	result := strconv.FormatFloat(val, 'f', 4, 32)
	return strings.TrimRight(strings.TrimRight(result, "0"), ".")
}

func getRandomTip() string {
	rand.Seed(time.Now().UnixNano())
	tipIndex := rand.Intn(len(tips))
	return tips[tipIndex]
}

func getRandomTgEmoji() string {
	rand.Seed(time.Now().UnixNano())
	tipIndex := rand.Intn(len(tgEmojiList))
	return tgEmojiList[tipIndex]
}

func LimitStringLength(str string, maxLength int) string {
	if len(str) > maxLength {
		return str[:maxLength] + ".."
	}
	return str
}

func filterNickname(nickname string) string {
	nickname = strconv.QuoteToASCII(nickname)
	nickname = strings.ReplaceAll(nickname, `"`, "")
	return LimitStringLength(nickname, nicknameMaxLength)
}

func (app *solution) genGameVoucher() string {
	return fmt.Sprintf(
		gameVoucherTemplate,
		app.Config.GameVoucherPrefix,
		strings.ToUpper(swissknife.GetRandomString(2)),
		strings.ToUpper(swissknife.GetRandomString(4)),
		strings.ToUpper(swissknife.GetRandomString(4)),
		strings.ToUpper(swissknife.GetRandomString(4)),
	)
}
