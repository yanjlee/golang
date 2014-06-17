package main

import (
	/*"bytes"*/
	"fmt"
	/*"io/ioutil"*/
        "strings"
	"os"
	/*"net/url"*/
	/*iconv "github.com/djimenez/iconv-go"*/
	"crypto/md5"
	"encoding/hex"
	/*"code.google.com/p/go.net/html"*/
	/*"github.com/momo/myhttp"*/
	/*"github.com/bitly/go-simplejson"*/
	"code.google.com/p/gcfg"
	"flag"
	"log"
	"os/signal"
        /*"sync"*/
	"syscall"
	/*"time"*/
	"strconv"
)


var (
	// basic options
	/*showVersion = flag.Bool("version", false, "print version string")*/
	verbose          = flag.Bool("verbose", false, "enable verbose logging")
	scantieba        = flag.Int("scantieba", 10000000, "scan tieba data")
	outPutConfigFile = flag.Bool("o", false, "generate the texas_monitor.ini file")
)

func main() {

	var cfg Config

	flag.Parse()

	if *outPutConfigFile {
		generalConfigFile()
		return
	}

        if *verbose {
           fmt.Println("verbose mode")
        }

	err := gcfg.ReadFileInto(&cfg, "./momo.ini")
	if err != nil {
		log.Fatalf("Failed to parse momo.ini: %s", err)
	}

        /*dup stdout*/
	f, err := os.OpenFile("momo_log.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	defer f.Close()

	log.SetOutput(f)

        momoManagerInt()


        /*exit signle*/
	exitChan := make(chan int)
	signalChan := make(chan os.Signal, 1)
	go func() {
		<-signalChan
		exitChan <- 1
	}()

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)


        setCfg(&cfg)

        locSession = NewLocalSession()

        locSession.LoadSessiondata()

        var accountNum int
        for _, momotmp := range cfg.Account.MoMoID {
           /*fmt.Println(momotmp)*/

           momo := strings.Split(momotmp, " ")
           if len(momo) == 2 {
              momoid := momo[0]
              pwd := momo[1]

              if momoManagerCheck(momoid) == true{
                 fmt.Println("momoid Duplicate: ", momoid)
                 continue
              }

              user := NewUser()
              user.id, _ = strconv.Atoi(momoid)
              user.idStr = momoid

              user.password = pwd

              if len(user.password) != 32 {
                 h := md5.New()
                 h.Write([]byte(pwd))
                 passwd := hex.EncodeToString(h.Sum(nil))
                 user.password = passwd
                 fmt.Printf("%d: password md5=%s\n",user.id, user.password)
              }else{
                 fmt.Printf("%d: password md5=%s\n",user.id, user.password)
              }

              momoManagerAdd(user)

              accountNum++
           }else{
              fmt.Println("账号配置错误:", momo, len(momo))
           }
        }

        if accountNum == 0 {
           fmt.Println("Err: 请配置账号密码")
           return
        }
        /*return*/

        momoManagerStart()


        <-exitChan

        momoManageExit()
	return

}
