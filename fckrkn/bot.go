package fckrkn

import (
	"fmt"
	"github.com/Syfaro/telegram-bot-api"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"
)

var (
	Verbose *log.Logger
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger
)

var reProxy = regexp.MustCompile(":\\/\\/|@")
var keys = map[interface{}]interface{}{}
var values = map[interface{}]interface{}{}
var strikesUser = map[interface{}]interface{}{}
var strikesProxy = map[interface{}]interface{}{}
var timeoutUser = map[interface{}]interface{}{}
var lock = sync.RWMutex{}
var amount = uint(0)

func inc(v *uint) {
	lock.Lock()
	defer lock.Unlock()
	(*v)++
}

func dec(v *uint) {
	lock.Lock()
	defer lock.Unlock()
	(*v)--
}

func writeDict(key interface{}, value interface{}, d *map[interface{}]interface{}) {
	lock.Lock()
	defer lock.Unlock()
	(*d)[key] = value
}

func delDict(key interface{}, d *map[interface{}]interface{}) {
	lock.Lock()
	defer lock.Unlock()
	delete((*d), key)
}

func readDict(key interface{}, d *map[interface{}]interface{}) interface{} {
	lock.RLock()
	defer lock.RUnlock()
	return (*d)[key]
}

func existsDict(key interface{}, d *map[interface{}]interface{}) interface{} {
	lock.RLock()
	defer lock.RUnlock()

	_, ok := (*d)[key]

	return ok
}

func Init(
	verboseHandle io.Writer,
	infoHandle io.Writer,
	warningHandle io.Writer,
	errorHandle io.Writer,
) {
	Verbose = log.New(verboseHandle,
		"VERBOSE: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Info = log.New(infoHandle,
		"INFO: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Warning = log.New(warningHandle,
		"WARNING: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Error = log.New(errorHandle,
		"ERROR: ",
		log.Ldate|log.Ltime|log.Lshortfile)
}

type bot struct {
	token string
	opts  *Options
}

func New(
	token string,
	opts *Options,
) bot {
	b := bot{token, opts}
	return b
}

func (b bot) put(db *leveldb.DB, update tgbotapi.Update, botApi *tgbotapi.BotAPI) {
	var reply, idx string
	proxyString := "socks5://" + update.Message.CommandArguments()
	success := false
	var err error
	var ok bool

	chatId := strconv.FormatInt(update.Message.Chat.ID, 10)
	if readDict(chatId, &timeoutUser) != nil &&
		readDict(chatId, &timeoutUser).(bool) {
		reply = fmt.Sprintf(
			"Please wait a little before calling again :) Timeout is equal to %d seconds.",
			b.opts.opTimeout,
		)
		goto ret
	}

	go UserWatcher(chatId, b.opts)

	_, _, _, _, _, err = ParseProxy(proxyString)
	if err != nil {
		Error.Printf(
			"%s\n\tProxy: %s\n\tUser: %s\n\tChatId: %s",
			err, proxyString, update.Message.From.UserName, chatId,
		)
		reply = "Can't add your proxy. Probably it's invalid."
		goto ret
	}

	err, ok = CheckProxy(proxyString, b.opts)
	if !ok {
		Warning.Printf("Proxy is invalid\n\tProxy: %s\n\t%s", proxyString, err)
		reply = "Can't add your proxy. Probably it's invalid."
		goto ret
	} else {
		Verbose.Printf("Proxy is valid\n\tProxy: %s", proxyString)
	}

	if amount == b.opts.capacity {
		Verbose.Printf(
			"Limit is reached, can't add new proxy\n\tProxy: %s\n\tAmount: %d\n\tUser: %s\n\tChatId: %s",
			proxyString, amount, update.Message.From.UserName, chatId,
		)
		reply = "Sorry, can't add more proxies, limit is reached :("
		goto ret
	}

	if readDict(proxyString, &values) == nil {
		writeDict(proxyString, true, &values)
	} else {
		Verbose.Printf(
			"Replicated proxy\n\tProxy: %s\n\tUser: %s\n\tChatId: %s",
			proxyString, update.Message.From.UserName, chatId,
		)
		reply = "Sorry, but someone else added this proxy already."
		goto ret
	}

	for i := uint(0); i < b.opts.capacity; i++ {
		idx = strconv.FormatUint(uint64(i), 10)

		if readDict(idx, &keys) == nil || !readDict(idx, &keys).(bool) {
			err = db.Put([]byte(idx), []byte(proxyString), nil)
			if err != nil {
				Error.Printf(
					"Can't put proxy to db\n\tProxy: %s\n\tIndex: %s\n\t%s\n\tUser: %s\n\tChatId: %s",
					proxyString, idx, update.Message.From.UserName, chatId, err,
				)
				goto ret
			} else {
				writeDict(idx, true, &keys)
				success = true
				inc(&amount)
				Verbose.Printf(
					"Put proxy successfully\n\tProxy: %s\n\tIndex: %s\n\tUser: %s\n\tChatId: %s",
					proxyString, idx, update.Message.From.UserName, chatId,
				)
				break
			}
		}
	}

	if success {
		reply = "We've added your server to our database! Thank you for supporting resistance!"
		go Watcher(idx, proxyString, b.opts, db)
	} else {
		reply = "Something went wrong while putting your server to database, sorry"
	}

ret:
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)
	msg.ParseMode = "markdown"
	_, err = botApi.Send(msg)
	if err != nil {
		Error.Printf(
			"Can't send reply to user\n\tUser: %s\n\tChatId: %s",
			update.Message.From.UserName, chatId,
		)
	}
}

func (b bot) get(db *leveldb.DB, update tgbotapi.Update, botApi *tgbotapi.BotAPI) {
	var reply, proxyString, idx, chatId, setupURL string
	var success bool = false
	var iter iterator.Iterator
	var idxUint uint
	var key []byte
	var markup tgbotapi.InlineKeyboardMarkup

	if amount == 0 {
		reply = "Nothing to show yet :\\"
		goto ret
	}

	chatId = strconv.FormatInt(update.Message.Chat.ID, 10)
	if readDict(chatId, &timeoutUser) != nil &&
		readDict(chatId, &timeoutUser).(bool) {
		reply = fmt.Sprintf(
			"Please wait a little before calling again :) Timeout is equal to %d seconds.",
			b.opts.opTimeout,
		)
		goto ret
	}

	go UserWatcher(chatId, b.opts)

	rand.Seed(time.Now().UTC().UnixNano())
	idxUint = uint(rand.Intn(int(amount)))

	iter = db.NewIterator(nil, nil)
	defer iter.Release()
	for iter.Next() {
		key = iter.Key()
		if idxUint <= 0 {
			break
		}
		idxUint--
	}
	idx = string(key)

	if readDict(idx, &keys) != nil && readDict(idx, &keys).(bool) {
		proxyBytes, err := db.Get([]byte(idx), nil)
		proxyString = string(proxyBytes)
		if err != nil {
			Error.Printf("Can't get proxy from db\n\tIndex: %s\n\t%s\n\tUser: %s\n\tChatId: %s",
				idx, err, update.Message.From.UserName, chatId,
			)
		} else {
			Verbose.Printf(
				"Successfully got proxy from db\n\tIndex: %s\n\tProxy: %s\n\tUser: %s\n\tChatId: %s",
				idx, proxyString, update.Message.From.UserName, chatId,
			)
			success = true
		}
	}

	if success {
		_, proxyHost, proxyPort, proxyUser, proxyPass, err := ParseProxy(proxyString)
		if err != nil {
			Error.Printf(
				"%s\n\tProxy: %s\n\tUser: %s\n\tChatId: %s\n\tIndex: %s",
				err, proxyString, update.Message.From.UserName, chatId, idx,
			)
			reply = fmt.Sprintf(
				"Can't parse proxy for some reason but it works :)\n*%s\nIndex: %s*",
				proxyString, idx,
			)
			success = false
		} else {
			if proxyUser != "" {
				reply = fmt.Sprintf(
					"*Host: %s\nPort: %s\nUser: %s\nPass: %s\nIndex: %s*",
					proxyHost, proxyPort, proxyUser, proxyPass, idx,
				)
				setupURL, _ = GetSetupProxyURL(proxyString)
			} else {
				reply = fmt.Sprintf("*Host: %s\nPort: %s\nIndex: %s*", proxyHost, proxyPort, idx)
				setupURL, _ = GetSetupProxyURL(proxyString)
			}
		}
	} else {
		reply = "Something went wrong while getting server from database, try one more time :)"
	}

ret:
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)
	msg.ParseMode = "markdown"
	if success {
		button := tgbotapi.NewInlineKeyboardButtonURL("Setup this proxy", setupURL)
		markup.InlineKeyboard = append(markup.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(
			button,
		))
		msg.ReplyMarkup = &markup
	}
	_, err := botApi.Send(msg)
	if err != nil {
		Error.Printf("Can't send reply to %s\n\t%s", update.Message.From.UserName, err)
	}
}

func (b bot) strike(db *leveldb.DB, update tgbotapi.Update, botApi *tgbotapi.BotAPI) {
	var reply string
	idx := update.Message.CommandArguments()
	uidx := fmt.Sprintf("%s-%s", idx, update.Message.From.UserName)

	chatId := strconv.FormatInt(update.Message.Chat.ID, 10)
	if readDict(chatId, &timeoutUser) != nil &&
		readDict(chatId, &timeoutUser).(bool) {
		reply = fmt.Sprintf(
			"Please wait a little before calling again :) Timeout is equal to %d seconds.",
			b.opts.opTimeout,
		)
		goto ret
	}

	go UserWatcher(chatId, b.opts)

	if readDict(idx, &keys) != nil && readDict(idx, &keys).(bool) {
		if readDict(uidx, &strikesUser) == nil || !readDict(uidx, &strikesUser).(bool) {
			writeDict(uidx, true, &strikesUser)
			v := readDict(idx, &strikesProxy)
			var count uint
			if v != nil {
				count = v.(uint)
			} else {
				count = 0
			}
			writeDict(idx, count+1, &strikesProxy)
			Verbose.Printf(
				"Struck successfully\n\tProxy: %s\n\tUser: %s\n\tChatId: %s",
				idx, update.Message.From.UserName, chatId,
			)
			reply = "We've considered your strike, thank you!"
		} else {
			reply = "You've already struck this proxy, slow down :) You can try again later."
			goto ret
		}
	}

ret:
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)
	msg.ParseMode = "markdown"
	_, err := botApi.Send(msg)
	if err != nil {
		Error.Printf(
			"Can't send reply to user\n\tUser: %s\n\tChatId: %s",
			update.Message.From.UserName, chatId,
		)
	}
}

func welcome(update tgbotapi.Update, botApi *tgbotapi.BotAPI) {
	reply := fmt.Sprintf(
		"*Hello and welcome to digital resistance!*\n\n" +

			"This bot handles only working proxies if you can't access some of them there's a chance Roskompozor blocked them. " +
			"In this situation you probably wanna report such proxies, do it with the /strike command.\n\n" +

			"*Commands you can use:*\n\n" +

			"/get - for gathering a random proxy\n" +
			"/put - for adding a new one\n" +
			"/strike - for reporting non working proxies\n\n" +

			"Adding new proxies requires a special form:\n" +
			"[user:password@]host:port\n" +
			"Example:\n" +
			"user:pass@someproxy.com:1080\n" +
			"or\n" +
			"someproxy.com:1080\n\n" +

			"To strike a proxy you have to pass index that you've got from the /get command.\n" +
			"Example:\n" +
			"/strike 1\n\n" +

			"You can also support the resistance by setting up your own proxy! It's really easy if you know what docker is :) " +
			"If you're outside Russia, you can set up a proxy for short period(s) of time on your own PC for example. " +
			"This docker image was made especially for proxying of Telegram.\n" +
			"https://hub.docker.com/r/schors/tgdante2\n" +
			"*Beta testing*",
	)

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)
	msg.ParseMode = "markdown"
	_, err := botApi.Send(msg)
	if err != nil {
		Error.Printf("Can't send reply to %s", update.Message.From.UserName)
	}
}

func StrikesReseter(opts *Options) {
	for {
		strikesUser = map[interface{}]interface{}{}
		strikesProxy = map[interface{}]interface{}{}
		Info.Printf("Strikes reset successfully")
		time.Sleep(time.Duration(opts.strikeTimeout) * time.Minute)
	}
}

func UserWatcher(chatId string, opts *Options) {
	writeDict(chatId, true, &timeoutUser)
	time.Sleep(time.Duration(opts.opTimeout) * time.Second)
	writeDict(chatId, false, &timeoutUser)
}

func Watcher(idx, proxyString string, opts *Options, db *leveldb.DB) {
	th := uint8(0)

	for {
		err, ok := CheckProxy(proxyString, opts)

		if !ok {
			th++
			Verbose.Printf(
				"Checkout failed\n\tProxy: %s\n\tIndex: %s\n\tAmount: %d\n\tTimes Failed: %d\n\t%s",
				proxyString, idx, amount, th, err,
			)
		} else {
			th = 0
			Verbose.Printf(
				"Checkout succeeded\n\tProxy: %s\n\tIndex: %s\n\tAmount: %d",
				proxyString, idx, amount,
			)
		}

		v := readDict(idx, &strikesProxy)
		var strikesCount uint
		if v != nil {
			strikesCount = v.(uint)
		} else {
			strikesCount = 0
		}

		if th > opts.checkCap || strikesCount >= opts.strikeCap {
			err = db.Delete([]byte(idx), nil)
			if err != nil {
				Error.Printf(
					"Can't delete proxy from db\n\tProxy: %s\n\tIndex: %s\n\t%s",
					proxyString,
					idx,
					err,
				)
			} else {
				delDict(idx, &keys)
				delDict(proxyString, &values)
				delDict(idx, &strikesProxy)
				dec(&amount)
				Info.Printf("Proxy removed by watcher\n\tProxy: %s\n\tIndex: %s", proxyString, idx)
				Verbose.Printf("Amount: %d", amount)
			}
			return
		}
		time.Sleep(time.Duration(opts.timeout) * time.Minute)
	}
}

func (b bot) Start() {
	if b.opts.verbose {
		Init(os.Stdout, os.Stdout, os.Stdout, os.Stderr)
	} else {
		Init(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr)
	}

	db, err := leveldb.OpenFile(b.opts.dbpath, nil)
	if err != nil {
		Error.Println("Can't get access to database")
		panic(err)
	}
	defer db.Close()

	iter := db.NewIterator(nil, nil)
	defer iter.Release()
	for iter.Next() {
		k, v := iter.Key(), iter.Value()
		keys[string(k)] = true
		values[string(v)] = true
		amount++
		go Watcher(string(k), string(v), b.opts, db)
		time.Sleep(time.Duration(10) * time.Millisecond)
	}

	botApi, err := tgbotapi.NewBotAPI(b.token)
	if err != nil {
		Error.Println("Can't authenticate with given token")
		panic(err)
	}

	Info.Printf("Authorized on account %s", botApi.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := botApi.GetUpdatesChan(u)
	go StrikesReseter(b.opts)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		Info.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		switch update.Message.Command() {
		case "start":
			go welcome(update, botApi)
		case "put":
			go b.put(db, update, botApi)
		case "get":
			go b.get(db, update, botApi)
		case "strike":
			go b.strike(db, update, botApi)
		}
	}
}
