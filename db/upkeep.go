package db

import (
	"log"
	"time"

	"github.com/bakape/captchouli/common"
)

// Time it takes for one captcha to expire
const expiryTime = 30 * time.Minute

func runUpkeepTasks() {
	go func() {
		min := time.Tick(time.Minute)
		hour := time.Tick(time.Hour)
		for {
			var err error
			select {
			case <-min:
				err = deleteStaleCaptchas()
			case <-hour:
				err = vacuum()
			}
			if err != nil {
				log.Println(common.Error{err})
			}
		}
	}()
}

func deleteStaleCaptchas() error {
	dbMu.Lock()
	defer dbMu.Unlock()

	_, err := sq.Delete("captchas").
		Where("created < ? ", time.Now().Add(-expiryTime).UTC()).
		Exec()
	return err
}

func vacuum() error {
	dbMu.Lock()
	defer dbMu.Unlock()

	_, err := db.Exec("vacuum")
	return err
}
