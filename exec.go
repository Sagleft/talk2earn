package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tb "github.com/Sagleft/telegobot"
)

func escapeExecArgs(command string) []string {
	return strings.Split(command, " ")

}

func getLogsByUserFile(pubkey string) (string, error) {
	fromTime := time.Now().Add(-1 * time.Hour * 24 * 5)
	args := escapeExecArgs(`-u bankbot.service --since ` + fromTime.Format(journalLogsTimeFormat))

	var out bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command("journalctl", args...)
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", errors.New(fmt.Sprint(err) + ": " + stderr.String())
	}

	strs := strings.Split(out.String(), "\n")
	result := ""
	for _, line := range strs {
		if strings.Contains(line, pubkey) {
			result += line + "\n"
		}
	}

	if out.String() == "" {
		return "", errors.New("empty logs")
	}

	return result, nil
}

func (app *solution) getLogsByUser(pubkey string, replyToTelegramUserID int64) error {
	fileData, err := getLogsByUserFile(pubkey)
	if err != nil {
		return err
	}

	if fileData == "" {
		_, err = app.TelegramBot.Send(tb.ChatID(replyToTelegramUserID), "логи не найдены")
		return err
	}

	reader := bytes.NewReader([]byte(fileData))
	_, err = app.TelegramBot.Send(tb.ChatID(replyToTelegramUserID), &tb.Document{
		File:     tb.FromReader(reader),
		MIME:     "text/plain",
		FileName: "log.txt",
	})
	return err
}
