package dataprovider

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/alexedwards/argon2id"

	"golang.org/x/crypto/bcrypt"

	"github.com/pilif/sftpgo/logger"
	"github.com/pilif/sftpgo/utils"
)

func getUserByUsername(username string) (User, error) {
	var user User
	q := getUserByUsernameQuery()
	stmt, err := dbHandle.Prepare(q)
	if err != nil {
		logger.Debug(logSender, "error preparing database query %v: %v", q, err)
		return user, err
	}
	defer stmt.Close()

	row := stmt.QueryRow(username)
	return getUserFromDbRow(row, nil)
}

func sqlCommonValidateUserAndPass(username string, password string) (User, error) {
	var user User
	if len(password) == 0 {
		return user, errors.New("Credentials cannot be null or empty")
	}
	user, err := getUserByUsername(username)
	if err != nil {
		logger.Warn(logSender, "error authenticating user: %v, error: %v", username, err)
	} else {
		var match bool
		if strings.HasPrefix(user.Password, argonPwdPrefix) {
			match, err = argon2id.ComparePasswordAndHash(password, user.Password)
			if err != nil {
				logger.Warn(logSender, "error comparing password with argon hash: %v", err)
				return user, err
			}

		} else if strings.HasPrefix(user.Password, bcryptPwdPrefix){
			err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
			if err != nil {
				logger.Warn(logSender, "error comparing password with bcrypt hash: %v", err)
				return user, err
			}else{
				match = true
			}
		} else {
			// clear text password match
			match = (user.Password == password)
		}
		if !match {
			err = errors.New("Invalid credentials")
		}
	}
	return user, err
}

func sqlCommonValidateUserAndPubKey(username string, pubKey string) (User, error) {
	var user User
	if len(pubKey) == 0 {
		return user, errors.New("Credentials cannot be null or empty")
	}
	user, err := getUserByUsername(username)
	if err != nil {
		logger.Warn(logSender, "error authenticating user: %v, error: %v", username, err)
		return user, err
	}
	if len(user.PublicKey) > 0 {
		storedPubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(user.PublicKey))
		if err != nil {
			logger.Warn(logSender, "error parsing stored public key for user %v: %v", username, err)
			return user, err
		}
		if string(storedPubKey.Marshal()) != pubKey {
			err = errors.New("Invalid credentials")
			return user, err
		}
	} else {
		err = errors.New("Invalid credentials")
	}
	return user, err
}

func sqlCommonGetUserByID(ID int64) (User, error) {
	var user User
	q := getUserByIDQuery()
	stmt, err := dbHandle.Prepare(q)
	if err != nil {
		logger.Debug(logSender, "error preparing database query %v: %v", q, err)
		return user, err
	}
	defer stmt.Close()

	row := stmt.QueryRow(ID)
	return getUserFromDbRow(row, nil)
}

func sqlCommonUpdateQuota(username string, filesAdd int, sizeAdd int64, reset bool, p Provider) error {
	var usedFiles int
	var usedSize int64
	var err error
	if reset {
		usedFiles = 0
		usedSize = 0
	} else {
		usedFiles, usedSize, err = p.getUsedQuota(username)
		if err != nil {
			return err
		}
	}
	usedFiles += filesAdd
	usedSize += sizeAdd
	if usedFiles < 0 {
		logger.Warn(logSender, "used files is negative, probably some files were added and not tracked, please rescan quota!")
		usedFiles = 0
	}
	if usedSize < 0 {
		logger.Warn(logSender, "used files is negative, probably some files were added and not tracked, please rescan quota!")
		usedSize = 0
	}

	q := getUpdateQuotaQuery()
	stmt, err := dbHandle.Prepare(q)
	if err != nil {
		logger.Debug(logSender, "error preparing database query %v: %v", q, err)
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(usedSize, usedFiles, utils.GetTimeAsMsSinceEpoch(time.Now()), username)
	if err == nil {
		logger.Debug(logSender, "quota updated for user %v, new files: %v new size: %v", username, usedFiles, usedSize)
	} else {
		logger.Warn(logSender, "error updating quota for username %v: %v", username, err)
	}
	return err
}

func sqlCommonGetUsedQuota(username string) (int, int64, error) {
	q := getQuotaQuery()
	stmt, err := dbHandle.Prepare(q)
	if err != nil {
		logger.Warn(logSender, "error preparing database query %v: %v", q, err)
		return 0, 0, err
	}
	defer stmt.Close()

	var usedFiles int
	var usedSize int64
	err = stmt.QueryRow(username).Scan(&usedSize, &usedFiles)
	if err != nil {
		logger.Warn(logSender, "error getting user quota: %v, error: %v", username, err)
		return 0, 0, err
	}
	return usedFiles, usedSize, err
}

func sqlCommonCheckUserExists(username string) (User, error) {
	var user User
	q := getUserByUsernameQuery()
	stmt, err := dbHandle.Prepare(q)
	if err != nil {
		logger.Warn(logSender, "error preparing database query %v: %v", q, err)
		return user, err
	}
	defer stmt.Close()
	row := stmt.QueryRow(username)
	return getUserFromDbRow(row, nil)
}

func sqlCommonAddUser(user User) error {
	err := validateUser(&user)
	if err != nil {
		return err
	}
	q := getAddUserQuery()
	stmt, err := dbHandle.Prepare(q)
	if err != nil {
		logger.Warn(logSender, "error preparing database query %v: %v", q, err)
		return err
	}
	defer stmt.Close()
	permissions, err := user.GetPermissionsAsJSON()
	if err != nil {
		return err
	}
	_, err = stmt.Exec(user.Username, user.Password, user.PublicKey, user.HomeDir, user.UID, user.GID, user.MaxSessions, user.QuotaSize,
		user.QuotaFiles, string(permissions), user.UploadBandwidth, user.DownloadBandwidth)
	return err
}

func sqlCommonUpdateUser(user User) error {
	err := validateUser(&user)
	if err != nil {
		return err
	}
	q := getUpdateUserQuery()
	stmt, err := dbHandle.Prepare(q)
	if err != nil {
		logger.Warn(logSender, "error preparing database query %v: %v", q, err)
		return err
	}
	defer stmt.Close()
	permissions, err := user.GetPermissionsAsJSON()
	if err != nil {
		return err
	}
	_, err = stmt.Exec(user.Password, user.PublicKey, user.HomeDir, user.UID, user.GID, user.MaxSessions, user.QuotaSize,
		user.QuotaFiles, permissions, user.UploadBandwidth, user.DownloadBandwidth, user.ID)
	return err
}

func sqlCommonDeleteUser(user User) error {
	q := getDeleteUserQuery()
	stmt, err := dbHandle.Prepare(q)
	if err != nil {
		logger.Warn(logSender, "error preparing database query %v: %v", q, err)
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(user.ID)
	return err
}

func sqlCommonGetUsers(limit int, offset int, order string, username string) ([]User, error) {
	users := []User{}
	q := getUsersQuery(order, username)
	stmt, err := dbHandle.Prepare(q)
	if err != nil {
		logger.Warn(logSender, "error preparing database query %v: %v", q, err)
		return nil, err
	}
	defer stmt.Close()
	var rows *sql.Rows
	if len(username) > 0 {
		rows, err = stmt.Query(username, limit, offset)
	} else {
		rows, err = stmt.Query(limit, offset)
	}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			u, err := getUserFromDbRow(nil, rows)
			// hide password and public key
			u.Password = ""
			u.PublicKey = ""
			if err == nil {
				users = append(users, u)
			} else {
				break
			}
		}
	}

	return users, err
}

func getUserFromDbRow(row *sql.Row, rows *sql.Rows) (User, error) {
	var user User
	var permissions sql.NullString
	var password sql.NullString
	var publicKey sql.NullString
	var err error
	if row != nil {
		err = row.Scan(&user.ID, &user.Username, &password, &publicKey, &user.HomeDir, &user.UID, &user.GID, &user.MaxSessions,
			&user.QuotaSize, &user.QuotaFiles, &permissions, &user.UsedQuotaSize, &user.UsedQuotaFiles, &user.LastQuotaScan,
			&user.UploadBandwidth, &user.DownloadBandwidth)

	} else {
		err = rows.Scan(&user.ID, &user.Username, &password, &publicKey, &user.HomeDir, &user.UID, &user.GID, &user.MaxSessions,
			&user.QuotaSize, &user.QuotaFiles, &permissions, &user.UsedQuotaSize, &user.UsedQuotaFiles, &user.LastQuotaScan,
			&user.UploadBandwidth, &user.DownloadBandwidth)
	}
	if err != nil {
		return user, err
	}
	if password.Valid {
		user.Password = password.String
	}
	if publicKey.Valid {
		user.PublicKey = publicKey.String
	}
	if permissions.Valid {
		var list []string
		err = json.Unmarshal([]byte(permissions.String), &list)
		if err == nil {
			user.Permissions = list
		}
	}
	return user, err
}
