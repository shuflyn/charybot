package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	tb "gopkg.in/telebot.v3"
)

var (
	conf struct {
		Bot_key     string   `json:"bot_key"`
		Bot_masters []string `json:"bot_master"`
	}

	msg struct {
		Start string `json:"start"`
	}

	// for some telebot function
//	fp struct {
//		Fp struct {
//			Fp string `json:"file_path"`
//		} `json:"result"`
//	}

// ok error
)

func init() {
	if conf_file, err := os.ReadFile("conf.json"); err == nil {
		if err := json.Unmarshal(conf_file, &conf); err == nil {
			log.Println("conf unjsoned.")
		} else {
			log.Fatalln("conf unjson fail: ", err.Error())
		}
	} else {
		log.Fatalln("conf open fail: ", err.Error())
	}

	if msg_file, err := os.ReadFile("msg.json"); err == nil {
		if err := json.Unmarshal(msg_file, &msg); err == nil {
			log.Println("msg unjsoned.")
		} else {
			log.Fatalln("msg unjson fail: ", err.Error())
		}
	} else {
		log.Fatalln("msg open fail: ", err.Error())
	}

	log.Println("init done.")
}

func lnr(c tb.Context, msg string) error {
	log.Println(fmt.Sprint(">> [", c.Message().Sender.ID, "]: "), msg)
	return c.Send(msg)
}

func main() {
	if logf, ok := os.OpenFile(fmt.Sprint("log/", time.Now().Unix(), ".txt"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666); ok == nil {
		defer logf.Close()
		multi := io.MultiWriter(logf, os.Stdout)
		log.SetOutput(multi)
		log.Println("Лог начат.")
	} else {
		log.Println(fmt.Sprint("Ошибка старта лога: ", ok.Error()))
		return
	}
	if b, ok := tb.NewBot(tb.Settings{Token: conf.Bot_key, Poller: &tb.LongPoller{Timeout: 10 * time.Second}}); ok == nil {
		log.Println(fmt.Sprint("Авторизация телеграм: ", b.Me.Username, " [", b.Me.ID, "]"))
		b.Handle(tb.OnText, func(c tb.Context) error {
			if c.Message().Sender.ID == 1540544703 {
				switch c.Message().Text {
				case "/start":
					return c.Send(fmt.Sprint("ваш Telegram ID: ", c.Message().Sender.ID, "\n\n", msg.Start))
				case "/pros":
					return c.Send("список проектов.")
				case "/jobs":
					return c.Send("список заданий.")
				default:
					return lnr(c, fmt.Sprint("Неизвестная команда: ", c.Message().Text))
				}
			} else {
				lnr(c, fmt.Sprint("ваш Telegram ID ( ", c.Message().Sender.ID, ") не подключен к боту."))
			}
			return nil
		})
		b.Start()
	} else {
		log.Println("Ошибка авторизации телеграм: ", ok.Error())
	}
}
