package main

/* to do in v 032

funcs:
slot gettion.
slot deletion.

*/

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
	tb "gopkg.in/telebot.v3"
)

const degree = 0.00899928005759539236861051115911

// one kilometer in earth-degreeses

var (
	conf struct {
		Bot_key    string `json:"bot_key"`
		Bot_master int64  `json:"bot_master"`

		SQL_host     string `json:"sql_host"`
		SQL_port     string `json:"sql_port"`
		SQL_name     string `json:"sql_name"`
		SQL_user     string `json:"sql_user"`
		SQL_pass     string `json:"sql_pass"`
		SQL_recreate string `json:"sql_recreate"`
		SQL_clear    string `json:"sql_clear"`
	}

	msg struct {
		Hello  string `json:"hello"`
		Start  string `json:"start"`
		Help   string `json:"help"`
		Help2  string `json:"help2"`
		Geo    string `json:"geo"`
		Info   string `json:"info"`
		Donate string `json:"donate"`
	}

	fp struct { // for some telebot function
		Fp struct {
			Fp string `json:"file_path"`
		} `json:"result"`
	}

	sqlb           *sql.DB
	sqlr           *sql.Rows
	ok             error
	mtx_newfile    sync.Mutex
	lat, long, rad float32
)

func init() {
	if conf_file, err := ioutil.ReadFile("conf.json"); err == nil {
		if err := json.Unmarshal(conf_file, &conf); err == nil {
			log.Println("conf unjsoned.")
		} else {
			log.Fatalln("conf unjson fail: ", err.Error())
		}
	} else {
		log.Fatalln("conf open fail: ", err.Error())
	}

	if msg_file, err := ioutil.ReadFile("msg.json"); err == nil {
		if err := json.Unmarshal(msg_file, &msg); err == nil {
			log.Println("msg unjsoned.")
		} else {
			log.Fatalln("msg unjson fail: ", err.Error())
		}
	} else {
		log.Fatalln("msg open fail: ", err.Error())
	}

	if sqlb, ok = sql.Open("postgres", fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", conf.SQL_host, conf.SQL_port, conf.SQL_user, conf.SQL_pass, conf.SQL_name)); ok == nil {
		if ok = sqlb.Ping(); ok == nil {
			log.Println("sql pinged.")
		} else {
			log.Fatalln("sql ping fail: ", ok.Error())
		}
	} else {
		log.Fatalln("sql open fail: ", ok.Error())
	}

	log.Println("init done.")
}

func lnr(c tb.Context, msg string) error {
	log.Println(fmt.Sprint(">> [", c.Message().Sender.ID, "]: "), msg)
	return c.Send(msg)
}

func sqlclear(c tb.Context) error {
	if sqlr, ok = sqlb.Query(conf.SQL_clear); ok == nil {
		sqlr.Close()
		return lnr(c, "sql cleared.")
	} else {
		return lnr(c, fmt.Sprint("sql clearing fail:", ok.Error()))
	}
}

func sqlrecreate(c tb.Context) error {
	if sqlr, ok = sqlb.Query(conf.SQL_recreate); ok == nil {
		sqlr.Close()
		return lnr(c, "sql recreated.")
	} else {
		return lnr(c, fmt.Sprint("sql recreation fail:", ok.Error()))
	}
}

func refold(c tb.Context) error {
	if ok = os.RemoveAll("ph"); ok == nil {
		lnr(c, "ph removed.")
		if ok = os.RemoveAll("dx"); ok == nil {
			lnr(c, "dx removed.")
			if ok = os.Mkdir("ph", 0777); ok == nil {
				lnr(c, "ph created.")
				if ok = os.Mkdir("dx", 0777); ok == nil {
					lnr(c, "dx created. done.")
					return nil
				} else {
					return lnr(c, fmt.Sprint("dx creating err: ", ok.Error()))
				}
			} else {
				return lnr(c, fmt.Sprint("ph creating err: ", ok.Error()))
			}
		} else {
			return lnr(c, fmt.Sprint("dx removing err: ", ok.Error()))
		}
	} else {
		return lnr(c, fmt.Sprint("ph removing err: ", ok.Error()))
	}
}

func newfile(c tb.Context, fnm string, fid string) error {
	go func() {
		if resp, ok := http.Get(fmt.Sprint("https://api.telegram.org/bot", conf.Bot_key, "/getFile?file_id=", fid)); ok == nil {
			defer resp.Body.Close()
			buf := new(bytes.Buffer)
			buf.ReadFrom(resp.Body)
			respb := buf.Bytes()
			if ok = json.Unmarshal(respb, &fp); ok == nil {
				mtx_newfile.Lock()
				if dir, ok := ioutil.ReadDir(fmt.Sprint("./", fnm, "/")); ok == nil {
					fnm = fmt.Sprint(fnm, "/", strconv.Itoa(len(dir)), ".", strings.Split(fp.Fp.Fp, ".")[1])
				}
				if resp, ok := http.Get(fmt.Sprint("https://api.telegram.org/file/bot", conf.Bot_key, "/", fp.Fp.Fp)); ok == nil {
					defer resp.Body.Close()
					if resp.StatusCode == 200 {
						if f, ok := os.Create(fnm); ok == nil {
							defer f.Close()
							if _, ok = io.Copy(f, resp.Body); ok == nil {
								if sqlr, ok = sqlb.Query(`insert into slotlist(uid, uniq) values($1, $2)`, c.Message().Sender.ID, fnm); ok == nil {
									sqlr.Close()
									lnr(c, fmt.Sprint("slot [", fnm, "] ok."))
								} else {
									lnr(c, fmt.Sprint("slotion error: ", ok))
								}
							} else {
								lnr(c, fmt.Sprint("upload error on ", fid))
							}
						} else {
							lnr(c, fmt.Sprint("file error on ", fnm))
						}
					} else {
						lnr(c, fmt.Sprint(resp.StatusCode, " respond on ", fid))
					}
				} else {
					lnr(c, fmt.Sprint("download error on ", fid))
				}
				mtx_newfile.Unlock()
			} else {
				lnr(c, fmt.Sprint("unjson error on ", fid))
			}
		} else {
			lnr(c, fmt.Sprint("getfile error on ", fid))
		}
	}()
	return nil
}

func list(c tb.Context, mode string, pg int) {
	brk := false
	switch mode {
	case "my":
		if sqlr, ok = sqlb.Query(`select count(uniq) from slotlist where uid = $1;`, c.Message().Sender.ID); ok != nil {
			brk = true
		}
	case "all":
		if sqlr, ok = sqlb.Query(`select count(uniq) from slotlist where uid is distinct from $1;`, c.Message().Sender.ID); ok != nil {
			brk = true
		}
	case "rad":
		if sqlr, ok = sqlb.Query(`select lat, long, rad from uidlist where uid = $1`, c.Sender().ID); ok == nil {
			defer sqlr.Close()
			for sqlr.Next() {
				sqlr.Scan(&lat, &long, &rad)
			}
			if sqlr, ok = sqlb.Query(`select count(slotlist.uniq) from slotlist inner join uidlist on slotlist.uid = uidlist.uid where (slotlist.uid is distinct from $1) and (lat between $2 and $3) and (long between $4 and $5);`, c.Message().Sender.ID, lat-(rad*degree), lat+(rad*degree), long-(rad*degree), long+(rad*degree)); ok != nil {
				brk = true
			}
		} else {
			lnr(c, fmt.Sprint("rad read fail: ", ok.Error()))
		}
	}
	defer sqlr.Close()
	if !brk {
		var cnt int
		for sqlr.Next() {
			sqlr.Scan(&cnt)
		}
		if cnt > 0 {
			switch mode {
			case "my":
				if sqlr, ok = sqlb.Query(`select uniq from slotlist where uid = $1 limit 10 offset $2;`, c.Message().Sender.ID, pg); ok != nil {
					brk = true
				}
			case "all":
				if sqlr, ok = sqlb.Query(`select uniq from slotlist where uid is distinct from $1 limit 10 offset $2;`, c.Message().Sender.ID, pg); ok != nil {
					brk = true
				}
			case "rad":
				if sqlr, ok = sqlb.Query(`select slotlist.uniq from slotlist inner join uidlist on slotlist.uid = uidlist.uid where (slotlist.uid is distinct from $1) and (lat between $2 and $3) and (long between $4 and $5) limit 10 offset $6;`, c.Message().Sender.ID, lat-(rad*degree), lat+(rad*degree), long-(rad*degree), long+(rad*degree), pg); ok != nil {
					brk = true
				}
			}
			defer sqlr.Close()
			if !brk {
				var lst, capt string
				for sqlr.Next() {
					sqlr.Scan(&lst)
					capt = strings.Replace(lst, "/", "", 1)
					capt = strings.Replace(capt, ".", "_", 1)
					if mode == "my" {
						capt = fmt.Sprint("удалить: /rem_", capt)
					} else {
						capt = fmt.Sprint("запрос: /get_", capt)
					}
					switch lst[0:2] {
					case "ph":
						var snd *tb.Photo
						snd = &tb.Photo{File: tb.FromDisk(fmt.Sprint("ph/", lst[2:])), Caption: capt}
						c.Send(snd)
					case "dx":
						var snd *tb.Document
						snd = &tb.Document{File: tb.FromDisk(fmt.Sprint("dx/", lst[2:])), Caption: capt}
						c.Send(snd)
					}
				}
				pg += 10
				if cnt > pg {
					c.Send(fmt.Sprint("/", mode, pg, " to list next 10"))
				} else {
					c.Send("end of list.")
				}
			} else {
				lnr(c, fmt.Sprint("listing error: ", ok))
			}
		} else {
			c.Send("any slot not found.")
		}
	} else {
		lnr(c, fmt.Sprint("count error: ", ok))
	}
}

func setloc(c tb.Context, lat float32, long float32) error {
	if sqlr, ok = sqlb.Query(`insert into uidlist(uid, lat, long, rad) values($1, $2, $3, 5);`, c.Message().Sender.ID, lat, long); ok == nil {
		defer sqlr.Close()
		lnr(c, fmt.Sprint("geo accepted: ", lat, ", ", long, "; rad set to 5km."))
	} else {
		if sqlr, ok = sqlb.Query(`update uidlist set lat= $2, long= $3 where uid= $1;`, c.Message().Sender.ID, lat, long); ok == nil {
			defer sqlr.Close()
			lnr(c, fmt.Sprint("geo accepted: ", lat, ", ", long))
		} else {
			lnr(c, fmt.Sprint("geo updation error: ", lat, ", ", long))
		}
	}
	return nil
}

func getloc(c tb.Context) string {
	if sqlr, ok = sqlb.Query(`select lat, long, rad from uidlist where uid = $1;`, c.Message().Sender.ID); ok == nil {
		for sqlr.Next() {
			if ok = sqlr.Scan(&lat, &long, &rad); ok == nil {
				return fmt.Sprint("geo: ", lat, ", ", long, "\nrad: ", rad, "km.")
			} else {
				lnr(c, fmt.Sprint("geo get fail: ", ok.Error()))
				return "geo get fail."
			}
		}
		return "sql read fail."
	} else {
		log.Println("geo gettion fail: ", ok.Error())
		return ok.Error()
	}
}

func setrad(c tb.Context, rad float32) {
	if sqlr, ok = sqlb.Query(`insert into uidlist(rad) values($2) where uid = $1`, c.Message().Sender.ID, rad); ok == nil {
		lnr(c, fmt.Sprint("rad set to ", rad, "km."))
	} else {
		log.Println("rad insertion fail: ", ok.Error())
		if sqlr, ok = sqlb.Query(`update uidlist set rad = $2 where uid = $1`, c.Message().Sender.ID, rad); ok == nil {
			lnr(c, fmt.Sprint("rad updated: ", rad, "km."))
		} else {
			lnr(c, fmt.Sprint("rad updation fail: ", ok.Error()))
		}
	}
}

func sendmsg(b tb.Bot, c tb.Context, smsg string) {
	var capt string = fmt.Sprint("запрос на /rem_", smsg, "\n\n")
	if smsg[0:2] == "ph" {
		smsg = fmt.Sprint("ph/", smsg[2:])
	} else if smsg[0:2] == "dx" {
		smsg = fmt.Sprint("dx/", smsg[2:])
	} else {
		return
	}
	smsg = strings.Replace(smsg, "_", ".", 1)
	if sqlr, ok = sqlb.Query(`select uid from slotlist where uniq = $1`, smsg); ok == nil {
		defer sqlr.Close()
		var suid int64
		for sqlr.Next() {
			sqlr.Scan(&suid)
		}
		if suid != 0 {
			if c.Chat().Username != "" {
				if c.Chat().FirstName != "" {
					capt = fmt.Sprint(capt, c.Chat().FirstName, "\n")
				}
				if c.Chat().LastName != "" {
					capt = fmt.Sprint(capt, c.Chat().LastName, "\n")
				}
				capt = fmt.Sprint(capt, "username: @", c.Chat().Username, "\n")
				switch smsg[0:2] {
				case "ph":
					var snd *tb.Photo = &tb.Photo{File: tb.FromDisk(fmt.Sprint("ph/", smsg[2:])), Caption: capt}
					b.Send(tb.ChatID(suid), snd)
					snd = &tb.Photo{File: tb.FromDisk(fmt.Sprint("ph/", smsg[2:])), Caption: "Запрос отправлен."}
					b.Send(tb.ChatID(c.Sender().ID), snd)
				case "dx":
					var snd *tb.Document = &tb.Document{File: tb.FromDisk(fmt.Sprint("dx/", smsg[2:])), Caption: capt}
					b.Send(tb.ChatID(suid), snd)
					snd = &tb.Document{File: tb.FromDisk(fmt.Sprint("dx/", smsg[2:])), Caption: "Запрос отправлен."}
					b.Send(tb.ChatID(c.Sender().ID), snd)
				}
			} else {
				capt = "Обращение через бота возможно только по username (Имя Пользователя Telegram),— как @shuflyn, установленное в приведённом изображении."
				var snd *tb.Photo = &tb.Photo{File: tb.FromDisk("username.jpg"), Caption: capt}
				b.Send(tb.ChatID(c.Sender().ID), snd)
			}
		} else {
			lnr(c, "user not found.")
		}
	}
}

func remslot(b tb.Bot, c tb.Context, smsg string) {
	if smsg[0:2] == "ph" {
		smsg = fmt.Sprint("ph/", smsg[2:])
	} else if smsg[0:2] == "dx" {
		smsg = fmt.Sprint("dx/", smsg[2:])
	} else {
		return
	}
	smsg = strings.Replace(smsg, "_", ".", 1)
	if sqlr, ok = sqlb.Query(`select uid from slotlist where uniq = $1`, smsg); ok == nil {
		defer sqlr.Close()
		var suid int64
		for sqlr.Next() {
			sqlr.Scan(&suid)
		}
		if suid == c.Sender().ID {
			if sqlr, ok = sqlb.Query(`delete from slotlist where uniq = $1;`, smsg); ok == nil {
				defer sqlr.Close()
				switch smsg[0:2] {
				case "ph":
					var snd *tb.Photo = &tb.Photo{File: tb.FromDisk(fmt.Sprint("ph/", smsg[2:])), Caption: "Объект удалён."}
					b.Send(tb.ChatID(c.Sender().ID), snd)
				case "dx":
					var snd *tb.Document = &tb.Document{File: tb.FromDisk(fmt.Sprint("dx/", smsg[2:])), Caption: "Объект удалён."}
					b.Send(tb.ChatID(c.Sender().ID), snd)
				}
			} else {
				lnr(c, fmt.Sprint("rem fail: ", ok.Error()))
			}
		} else {
			lnr(c, "be careful, please.")
		}
	} else {
		lnr(c, fmt.Sprint("somth wrong: ", ok.Error()))
	}
}

func geostop(c tb.Context) {
	if sqlr, ok = sqlb.Query(`delete from uidlist where uid = $1;`, c.Sender().ID); ok == nil {
		defer sqlr.Close()
		lnr(c, "Данные успешно удалены.")
	} else {
		lnr(c, fmt.Sprint("Ошибка при попытке очистки данных: ", ok.Error()))
	}
}

func main() {
	defer sqlb.Close()
	if logf, ok := os.OpenFile(fmt.Sprint("log/", time.Now().Unix(), ".txt"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666); ok == nil {
		defer logf.Close()
		multi := io.MultiWriter(logf, os.Stdout)
		log.SetOutput(multi)
		log.Println("logfile started.")
	} else {
		log.Println(fmt.Sprint("logfile start fail: ", ok.Error()))
		return
	}
	if b, err := tb.NewBot(tb.Settings{Token: conf.Bot_key, Poller: &tb.LongPoller{Timeout: 10 * time.Second}}); err == nil {
		log.Println(fmt.Sprint("tg auth: ", b.Me.Username, " [", b.Me.ID, "]"))
		b.Handle(tb.OnText, func(c tb.Context) error {
			switch c.Message().Text {
			case "/start":
				if c.Message().Text == "/start" {
					c.Send(msg.Start)
				}
				return c.Send(msg.Help)
			case "/my":
				go list(c, "my", 0)
				return nil
			case "/all":
				go list(c, "all", 0)
				return nil
			case "/geo":
				return c.Send(fmt.Sprint(msg.Geo, getloc(c)))
			case "/info":
				return c.Send(msg.Info)
			case "/donate":
				return c.Send(msg.Donate)
			case "/help":
				return c.Send(msg.Help2)
			case "/geoset":
				return c.Send("usage:\n/geoset latitude, longitude\n\nexample:\n/geoset 59.85619, 30.376776")
			case "/georad":
				return c.Send("usage:\n/georad kilometers\n\nexample:\n/georad 3")
			case "/list":
				go list(c, "rad", 0)
			case "/geostop":
				go geostop(c)
				return nil
			default:
				if len(c.Message().Text) > 2 {
					if c.Message().Text[0:3] == "/my" {
						if pg, ok := strconv.Atoi(c.Message().Text[3:]); ok == nil {
							go list(c, "my", pg)
							return nil
						}
					} else if len(c.Message().Text) > 3 {
						if c.Message().Text[0:4] == "/all" {
							if pg, ok := strconv.Atoi(c.Message().Text[4:]); ok == nil {
								go list(c, "all", pg)
								return nil
							}
						} else if c.Message().Text[0:4] == "/rad" {
							if pg, ok := strconv.Atoi(c.Message().Text[4:]); ok == nil {
								go list(c, "rad", pg)
								return nil
							}
						} else if len(c.Message().Text) > 5 {
							if c.Message().Text[0:5] == "/get_" {
								go sendmsg(*b, c, c.Message().Text[5:])
								return nil
							} else if c.Message().Text[0:5] == "/rem_" {
								go remslot(*b, c, c.Message().Text[5:])
								return nil
							} else if len(c.Message().Text) > 8 {
								if c.Message().Text[0:8] == "/geoset " {
									geo := strings.Split(strings.ReplaceAll(c.Message().Text[8:], " ", ""), ",")
									if lat, ok := strconv.ParseFloat(geo[0], 32); ok == nil {
										if long, ok := strconv.ParseFloat(geo[1], 32); ok == nil {
											go setloc(c, float32(lat), float32(long))
											return nil
										} else {
											lnr(c, fmt.Sprint("longitude fail: ", long))
											return nil
										}
									} else {
										lnr(c, fmt.Sprint("latitude fail: ", lat))
										return nil
									}
								} else if c.Message().Text[0:8] == "/georad " {
									if qrad, ok := strconv.ParseFloat(c.Message().Text[8:len(c.Message().Text)], 32); ok == nil {
										rad = float32(qrad)
										go setrad(c, rad)
										lnr(c, fmt.Sprint("/georad ", c.Message().Text[8:len(c.Message().Text)]))
										return nil
									} else {
										lnr(c, fmt.Sprint("rad convertion fail: ", ok.Error()))
									}
								}
							}
						}
					}
				}
				if c.Message().Sender.ID == conf.Bot_master {
					switch c.Message().Text {
					case "/sqlclear":
						return sqlclear(c)
					case "/sqlrecreate":
						return sqlrecreate(c)
					case "/refold":
						return refold(c)
					case "/close":
						go func() {
							time.Sleep(time.Second * 3)
							os.Exit(0)
						}()
						return lnr(c, "closing in 3 seconds.")
					default:
						return lnr(c, fmt.Sprint("unknown command from master: ", c.Message().Text))
					}
				} else {
					return lnr(c, fmt.Sprint("unknown command: ", c.Message().Text))
				}
			}
			return nil
		})
		b.Handle(tb.OnPhoto, func(c tb.Context) error {
			go newfile(c, "ph", c.Message().Photo.FileID)
			return nil
		})
		b.Handle(tb.OnChannelPost, func(c tb.Context) error {
			c.Send("hello")
			return lnr(c, fmt.Sprint("channel [", c.Chat().ID, "] post: ", c.Message().Text))
		})
		b.Handle(tb.OnDocument, func(c tb.Context) error {
			go newfile(c, "dx", c.Message().Document.FileID)
			return nil
		})
		b.Handle(tb.OnLocation, func(c tb.Context) error {
			go setloc(c, c.Message().Location.Lat, c.Message().Location.Lng)
			return nil
		})
		b.Start()
	} else {
		log.Println("cant auth.")
	}
}
