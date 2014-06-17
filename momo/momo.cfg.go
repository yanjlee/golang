
package main

import (
	/*"bytes"*/
	"fmt"
	"io/ioutil"
	/*"strings"*/
        "os"
	/*"net/url"*/
	/*iconv "github.com/djimenez/iconv-go"*/
	/*"crypto/md5"*/
	/*"encoding/hex"*/
	/*"code.google.com/p/go.net/html"*/
	/*"github.com/momo/myhttp"*/
        "github.com/bitly/go-simplejson"
        "encoding/json"
	/*"code.google.com/p/gcfg"*/
	/*"flag"*/
	"log"
	/*"os/signal"*/
	/*"sync"*/
	/*"syscall"*/
        "time"
)

type phone struct {
   mac         string
   uuid        string
   rom         string
   model       string
   buildnumber string
}

type apk struct {
   apksign   string
   version   string
   osversion string
}

type config struct {
        phone
        apk

	comment_count int //发贴条件  评论小于该值
	postTimeLimit int //发贴条件  主贴发送时间为该值时间内

	scanTimeLimit time.Duration //扫描贴子时间间隔
	postInterval  time.Duration
	/*waitGroup sync.WaitGroup*/
	postContent      []string //发送的内容
	tiebaId     []string //扫描的贴吧
	pics            []string  //发送的图片
}

var postCfg config

type Config struct {
	Phone struct {
		Mac         string
		UUID        string
		Rom         string
		Model       string
		Buildnumber string
	}

	Apk struct {
		Apksign   string
		Version   string
		Osversion string
	}

	Account struct {
		MoMoID        []string
		Password      []string
		PSWMD5        string
		Session       string
	}

	ActionCfg struct {
                 CommentCount  int
                 PostTimeLimit int
                 ScanTimeLimit int
                 PostInterval  int
	}
	PostContent struct {
		TiebaID []string
                 Content  []string
	}

	Pics struct {
	   Pic    []string
	}

	Mail struct {
		Addr []string
	}
}

func generalConfigFile() {
	b := []byte(
		`
[Phone]
Mac=CC:3A:61:17:36:F5                            #mac地址
UUID=431563936656c8edd53051ff27554975            #机器uuid，该值可以让手机客户端同时在线
Rom=4.2.2                                        #4.2.2
Model=SPH-L720                                   #SPH-L720
Buildnumber=JDQ39/L720VPUAMF9                    #JDQ39/L720VPUAMF9

[Apk]
Apksign=4f3a531caff3e37c278659cc78bfaecc         #4f3a531caff3e37c278659cc78bfaecc
Version=117                                      #117
Osversion=17                                     #17

[Account]
MoMoID=               #陌陌id 空格 密码（或密码MD5）   如:   账号是12345678, 密码是1234：12345678 1234即可，或12345678 81dc9bdb52d04dc20036dbd8313ed055

[ActionCfg]
CommentCount=5        #回复贴子条件  评论数<=该值
PostTimeLimit=30      #回复贴子条件  主贴发送时间为该值时间内 单位分钟

ScanTimeLimit=30      #扫描贴子时间间隔 单位秒
PostInterval=15       #发贴间隔  单位秒
[PostContent]
TiebaID=10000211
Content=你好              #发贴内容
Content=hello              #发贴内容


[Pics]
Pic=1.jpg              #图片



[Mail]
Addr=138@139.com  #告警通知邮件(可多个)
Addr=137@139.com

`)
	err := ioutil.WriteFile("./momo.ini", b, 0644)
	if err != nil {
		log.Fatalf("Failed to general texas_monitor.ini: %s", err)
		return
	}
	fmt.Println("general ./momo.ini successful")
}


func setCfg(cfg *Config) {

        postCfg.mac = cfg.Phone.Mac
        postCfg.uuid = cfg.Phone.UUID
        postCfg.rom = cfg.Phone.Rom
        postCfg.model = cfg.Phone.Model
        postCfg.buildnumber = cfg.Phone.Buildnumber
        postCfg.apksign = cfg.Apk.Apksign
        postCfg.version = cfg.Apk.Version
        postCfg.osversion = cfg.Apk.Osversion

	postCfg.comment_count = cfg.ActionCfg.CommentCount
	postCfg.postTimeLimit = cfg.ActionCfg.PostTimeLimit

	postCfg.postInterval = time.Duration(cfg.ActionCfg.PostInterval)
	postCfg.scanTimeLimit = time.Duration(cfg.ActionCfg.ScanTimeLimit)
	postCfg.postContent = cfg.PostContent.Content
	postCfg.tiebaId = cfg.PostContent.TiebaID
	postCfg.pics = cfg.Pics.Pic

        /*fmt.Printf("%#v\n", postCfg)*/
        fmt.Println("config:")

        fmt.Println("mac:           ", cfg.Phone.Mac)
        fmt.Println("uuid:          ", cfg.Phone.UUID)
        fmt.Println("rom:           ", cfg.Phone.Rom)
        fmt.Println("model:         ", cfg.Phone.Model)
        fmt.Println("buildnumber:   ", cfg.Phone.Buildnumber)
        fmt.Println("apksign:       ", cfg.Apk.Apksign)
        fmt.Println("version:       ", cfg.Apk.Version)
        fmt.Println("osversion:     ", cfg.Apk.Osversion)
        fmt.Println("comment_count: ", cfg.ActionCfg.CommentCount)
        fmt.Println("postTimeLimit: ", cfg.ActionCfg.PostTimeLimit)
        fmt.Println("postInterval:  ", cfg.ActionCfg.PostInterval)
        fmt.Println("scanTimeLimit: ", cfg.ActionCfg.ScanTimeLimit)
	/*fmt.Println("postcontent:   ", cfg.PostContent.Content)*/
	fmt.Println("tiebaID:   ", cfg.PostContent.TiebaID)

        for i, c:= range postCfg.postContent {
           fmt.Println("发送内容", i, ": ", c)
        }

}

type localSession struct {
   session map[string]string
}

var locSession *localSession

func NewLocalSession () *localSession {

   s := &localSession{
      session: make(map[string]string),
   }

   return s
}

func (s *localSession) PersistSessiondata() error {
        fileName := fmt.Sprintf("./momo.dat")

        log.Printf("MOMO: persisting session data to %s", fileName)

        js := make(map[string]interface{})
        users := make([]interface{}, 0)
        for _, user := range UserMap {
                userData := make(map[string]interface{})
                userData["id"] = user.idStr
                userData["session"] = user.session
                users = append(users, userData)
        }
        /*js["version"] = util.BINARY_VERSION*/
        js["users"] = users

        data, err := json.Marshal(&js)
        if err != nil {
                return err
        }

        tmpFileName := fileName + ".tmp"
        f, err := os.OpenFile(tmpFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
        if err != nil {
                return err
        }

        _, err = f.Write(data)
        if err != nil {
                f.Close()
                return err
        }
        f.Sync()
        f.Close()

        err = os.Rename(tmpFileName, fileName)
        if err != nil {
                return err
        }

        return nil
}


func (s *localSession) LoadSessiondata() {
        fn := fmt.Sprintf("momo.dat")
        data, err := ioutil.ReadFile(fn)
        if err != nil {
                if !os.IsNotExist(err) {
                        fmt.Printf("ERROR: failed to read Session data from %s - %s", fn, err.Error())
                }
                return
        }

        js, err := simplejson.NewJson(data)
        if err != nil {
                fmt.Printf("ERROR: failed to parse session data - %s", err.Error())
                return
        }

        users, err := js.Get("users").Array()
        if err != nil {
                fmt.Printf("ERROR: failed to parse session data - %s", err.Error())
                return
        }

        for user := range users {
                userJs := js.Get("users").GetIndex(user)

                idStr, err := userJs.Get("id").String()
                if err != nil {
                        fmt.Printf("ERROR: failed to parse session data - %s", err.Error())
                        return
                }

                session, err := userJs.Get("session").String()
                if err != nil {
                        fmt.Printf("ERROR: failed to parse session data - %s", err.Error())
                        return
                }

                if _, ok:= s.session[idStr]; ok == true {
                   continue
                }

                s.session[idStr] = session

                /*fmt.Printf("load session: '%s' : '%s'\n", idStr, session )*/
        }
}
