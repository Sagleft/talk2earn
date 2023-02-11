package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/logger"
	simplecron "github.com/sagleft/simple-cron"
)

func (app *solution) sqlDBConnect() error {
	logger.Info("connect to db..")
	var err error
	app.DB, err = newDBHandler(app.Config.DB)
	return err
}

func isSQLErrNoRows(err error) bool {
	return err == sql.ErrNoRows || strings.Contains(err.Error(), "no rows in result set")
}

func newDBHandler(task dbConnectionTask) (*dbHandler, error) {
	print("creating new db handler..")

	if task.UsersTable == "" {
		return nil, errors.New("users table is not set in `" + configJSONPath + "`")
	}

	var err error
	mysqlPort := task.Port
	if mysqlPort == "" {
		mysqlPort = "3306"
	}

	// username:password@tcp(127.0.0.1:3306)/dbname
	dsn := task.User + ":" + task.Pass + "@" +
		"tcp(" + task.Host + ":" + mysqlPort + ")" +
		"/" + task.DB

	var conn *sql.DB
	var connErr error
	isTimeIsUP := simplecron.NewRuntimeLimitHandler(
		sqldbConnectionTimeout,
		func() {
			conn, err = sql.Open(
				dbDriver, dsn,
			)
			if err != nil {
				connErr = errors.New("failed to open sqldb connection: " + err.Error())
			}
		},
	).Run()
	if connErr != nil {
		return nil, connErr
	}
	if isTimeIsUP {
		return nil, errors.New("the connection to the database went into timeout")
	}

	err = conn.Ping()
	if err != nil {
		return nil, errors.New("failed to connect to db: " + err.Error())
	}

	printSuccess("db handler created")
	return &dbHandler{
		Conn:       conn,
		UsersTable: task.UsersTable,
	}, nil
}

func (db *dbHandler) getUserData(pubkey, nickname string) (*userData, error) {
	user, err := db.getUserDBData(pubkey)
	if err != nil {
		return nil, err
	}
	if user != nil {
		return user, nil
	}

	u := userData{
		Pubkey:   pubkey,
		NickName: nickname,
	}
	err = db.saveUser(&u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *dbHandler) getUserDBData(pubkey string) (*userData, error) {
	user := &userData{
		Pubkey: pubkey,
	}
	sqlQuery := "SELECT uid,greed,nickname FROM " + db.UsersTable + " WHERE pubkey=? LIMIT 1"
	err := db.Conn.QueryRow(sqlQuery, pubkey).Scan(&user.UID, &user.Balance, &user.NickName)
	if err != nil {
		if isSQLErrNoRows(err) {
			return nil, nil
		}
		return nil, errors.New("failed to select user data: " + err.Error())
	}

	return user, nil
}

func (db *dbHandler) saveUser(user *userData) error {
	sqlQuery := "INSERT INTO " + db.UsersTable + " SET pubkey=?, nickname=?"
	result, err := db.Conn.Exec(sqlQuery, user.Pubkey, user.NickName)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.New("failed to get rows affected count: " + err.Error())
	}
	if rowsAffected == 0 {
		return errors.New("failed to save user to db, 0 rows affected")
	}
	return nil
}

func (db *dbHandler) addUserPoints(points float64, pubkey string) error {
	sqlQuery := "UPDATE " + db.UsersTable + " SET greed=greed+? WHERE pubkey=?"
	result, err := db.Conn.Exec(sqlQuery, points, pubkey)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.New("failed to get rows affected count: " + err.Error())
	}
	if rowsAffected == 0 {
		logger.Error(fmt.Errorf("failed add user points, 0 rows affected at user pubkey %s", pubkey))
	}
	return nil
}

func (db *dbHandler) resetUserPoints(pubkey string) error {
	sqlQuery := "UPDATE " + db.UsersTable + " SET greed=0 WHERE pubkey=?"
	result, err := db.Conn.Exec(sqlQuery, pubkey)
	if err != nil {
		if isSQLErrNoRows(err) {
			return errors.New("user not found")
		}
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.New("failed to get rows affected count: " + err.Error())
	}
	if rowsAffected == 0 {
		return errors.New("failed reset user points, 0 rows affected")
	}
	return nil
}

func (db *dbHandler) setUserPoints(pubkey string, points float64) error {
	sqlQuery := "UPDATE " + db.UsersTable + " SET greed=? WHERE pubkey=?"
	result, err := db.Conn.Exec(sqlQuery, points, pubkey)
	if err != nil {
		if isSQLErrNoRows(err) {
			return errors.New("user not found")
		}
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.New("failed to get rows affected count: " + err.Error())
	}
	if rowsAffected == 0 {
		return errors.New("failed set user points, 0 rows affected")
	}
	return nil
}

func (db *dbHandler) updateUserNickname(pubkey, newNickname string) error {
	sqlQuery := "UPDATE " + db.UsersTable + " SET nickname=? WHERE pubkey=?"
	_, err := db.Conn.Exec(sqlQuery, LimitStringLength(newNickname, nicknameMaxLength), pubkey)
	if err != nil {
		if isSQLErrNoRows(err) {
			return nil
		}
		logger.Error("failed to update user nickname: " + newNickname)
		return err
	}
	return nil
}

// pubkey -> nickname
type updateNicknameTask map[string]string

func (db *dbHandler) updateNicknames(task updateNicknameTask) error {
	for pubkey, nick := range task {
		if err := db.updateUserNickname(pubkey, nick); err != nil {
			return err
		}
	}
	return nil
}

func (db *dbHandler) saveGameVoucher(voucherCode string, pointsAmount float64) error {
	sqlQuery := "INSERT INTO game_vouchers SET code=?, amount=?"
	result, err := db.Conn.Exec(sqlQuery, voucherCode, pointsAmount)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.New("failed to get rows affected count: " + err.Error())
	}
	if rowsAffected == 0 {
		return errors.New("failed to save voucher to db, 0 rows affected")
	}
	return nil
}

func (db *dbHandler) deleteGameVoucher(voucherCode string) error {
	result, err := db.Conn.Exec("DELETE FROM game_vouchers WHERE code=?", voucherCode)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.New("failed to get rows affected count: " + err.Error())
	}
	if rowsAffected == 0 {
		return errors.New("voucher not found")
	}
	return nil
}

func (db *dbHandler) getGameVoucherAmount(voucherCode string) (float64, error) {
	var voucherAmount float64

	if err := db.Conn.QueryRow("SELECT amount FROM game_vouchers WHERE code=?", voucherCode).Scan(&voucherAmount); err != nil {
		if isSQLErrNoRows(err) {
			return 0, nil
		}
		return 0, err
	}

	return voucherAmount, nil
}
