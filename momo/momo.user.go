package main

import (
	"bytes"
	"code.google.com/p/go.net/html"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/bitly/go-simplejson"
	iconv "github.com/djimenez/iconv-go"
        "github.com/momo/myhttp"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"
	"errors"
)



type tiebaPostData struct {
	tid           int
	pid           int
	comment_count int
	title         string
}

type User struct {
	id           int
	idStr        string
	session      string
	password     string
	cookieJar    http.CookieJar
	waitGroup    *sync.WaitGroup
	postDataChan chan *tiebaPostData
	exitChan     chan int
	postPidMap   map[int]int
	content      []string //发送的内容
	postNum      int
	errCount     int      //多次失败后自动下线
}

func (u *User) Wrap(cb func()) {
	u.waitGroup.Add(1)
	go func() {
		cb()
		u.waitGroup.Done()
	}()
}

func NewUser() *User{
	user := &User{
		id:           0,
                idStr:        "",
		session:      "",
		cookieJar:    myhttp.NewJar(),
		waitGroup:    new(sync.WaitGroup),
		postDataChan: make(chan *tiebaPostData, 1000),
		exitChan:     make(chan int),
		postPidMap:   make(map[int]int),
		content:      []string{},
	}

        return user
}




func (u *User) Action(usermap *sync.WaitGroup) {

	usermap.Add(1)

	randNum := rand.Intn(4000)

        fmt.Printf("momoid:%d working\n", u.id)

        err := u.login(u.cookieJar)
        if err != nil {
           fmt.Println(err)

           momoManagerDel(u.idStr)

           goto finish
        }

	u.Wrap(func() { u.TiebaPostLoopScan() })

	u.TiebaPostScan()

	rand.Seed(time.Now().UnixNano())

	for {
		select {
		case <-u.exitChan:
			goto finish

		case <-time.After(postCfg.postInterval*time.Second + time.Duration(randNum)*time.Millisecond):
			if len(u.postDataChan) != 0 {
				pd := <-u.postDataChan
				u.tiebaPost(pd)
				randNum = rand.Intn(4000)
			}
		}
	}

finish:

      usermap.Done()
	/*for i := 0; i < 300; i++ {*/
	/*u.Tieba(i+tiebaidstart, u.cookieJar)*/
	/*}*/
}

func (u *User) Exit() {
	close(u.exitChan)
	u.waitGroup.Wait()
	fmt.Println("\nuser:", u.id, " exit")
}

func (u *User) login(cookieJar http.CookieJar) error {

	var loginAddr = "https://api.immomo.com/api/v2/login"
        /*fmt.Println(u)*/
        /*return errors.New("测试 不登录")*/

	if u.session == "" {

           if s, ok:=locSession.session[u.idStr]; ok{

              u.session = s

              if *verbose {
                 fmt.Println("load localsession success", u.id, " ", s)
              }
           }
        }

	if u.session != "" {
		url, _ := url.Parse(loginAddr)

		var sessionCookies = []*http.Cookie{
			{Name: "SESSIONID", Value: u.session},
		}

		cookieJar.SetCookies(url, sessionCookies)

		/*fmt.Println("sessionid:", u.session)*/
		return nil
	}

	if len(u.password) != 32 {
           return  errors.New("password len error!")
	}

        if *verbose {
           fmt.Printf("%d login use password: %s\n", u.id, u.password)
        }

	loginParams := map[string]string{
		"uid":           postCfg.uuid,
		"screen":        "1080x1920",
		"model":         postCfg.model,
		"rom":           postCfg.rom,
		"phone_type":    "GSM",
		"device_type":   "android",
		"emu":           "0",
		"mac":           postCfg.mac,
		"buildnumber":   postCfg.buildnumber,
		"password":      u.password,
		"apksign":       postCfg.apksign,
		"version":       postCfg.version,
		"phone_netWork": "2",
		"osversion_int": postCfg.osversion,
		"account":       fmt.Sprintf("%d", u.id),
		"gapps":         "1",
		"imsi":          "unknown",
		"market_source": "1",
		"etype":         "2",
	}

	config, err := myhttp.MultipartFormPost(loginAddr, loginParams, "", "", cookieJar)
	if err != nil {
		fmt.Println("err:", err)
                return  errors.New("myhttp.MultipartFormPost err")
	}

	js, err := simplejson.NewJson(config)
	if err != nil {
		fmt.Println("err:", err)
                return  errors.New("myhttp.MultipartFormPost err")
	}

	/*fmt.Printf("js: %#v\n", js)*/
	_, ok := js.CheckGet("ec")
	if ok {
		s, err := js.Get("ec").Int()
		if err != nil {
			fmt.Println("decode json err: get 'ec' failed!")
                        return  errors.New("decode json err")
		}
		if s == 0 {
			session, _ := js.Get("data").Get("session").String()

			/*fmt.Println(session)*/

			u.session = session

			u, _ := url.Parse(loginAddr)

			var sessionCookies = []*http.Cookie{
				{Name: "SESSIONID", Value: session},
			}

			cookieJar.SetCookies(u, sessionCookies)

                        locSession.PersistSessiondata()

			return nil
		} else {
			em, _ := js.Get("em").String()

			fmt.Println("login err: ", em)

                        return  errors.New("login err"+em)
		}
	}

        return  errors.New("login err: get ec err")
}

//10000187
func (u *User) Tieba(tid int, cookieJar http.CookieJar) {

	fmt.Println("tieba", tid)

	var tiebaAddr = "https://api.immomo.com/api/tieba/profile?fr=" + fmt.Sprintf("%d", u.id)

	loginParams := map[string]string{
		"tid": fmt.Sprintf("%d", tid),
	}

        jsdata, err := myhttp.MultipartFormPost(tiebaAddr, loginParams, "", "", cookieJar)
	if err != nil {
		fmt.Println("err:", err)
                return
	}

	js, err := simplejson.NewJson(jsdata)
	if err != nil {
		fmt.Println("err:", err)
		return
	}

	/*fmt.Printf("js: %#v", js)*/
	_, ok := js.CheckGet("ec")
	if ok {
		ec, err := js.Get("ec").Int()
		if err != nil {
			fmt.Println("decode Tieba json err: get 'ec' failed!")
			return
		}
		if ec == 0 {
			category, _ := js.Get("data").Get("category").Int()
			create_time, _ := js.Get("data").Get("create_time").Int64()
			name, _ := js.Get("data").Get("name").String()
			desc, _ := js.Get("data").Get("desc").String()
			member_count, _ := js.Get("data").Get("member_count").Int()
			tid, _ := js.Get("data").Get("tid").String()
			new_post_count, _ := js.Get("data").Get("new_post_count").Int()
			post_count, _ := js.Get("data").Get("post_count").Int()
			recommend, _ := js.Get("data").Get("recommend").Int()

			fmt.Printf("tid:%6s 类别:%03d 创建时间:%15s 总人数:%07d  post_count:%06d  new_post_count:%05d  recommend:%d 名称:%-9s 描述:%-13s\n",
				tid, category, time.Unix(create_time, 0), member_count, post_count, new_post_count, recommend, name, desc)
			return
		} else {
			em, _ := js.Get("em").String()

			fmt.Println("get teiba data err: ", em)

			return
		}
	}

	return

}

func (u *User) tiebaPost(pd *tiebaPostData) {

	var postAddr = "https://api.immomo.com/api/tieba/comment/publish?fr=" + fmt.Sprintf("%d", u.id)

        num := len(postCfg.postContent)
        if num == 0 {
           fmt.Println("post err: content is nil")
           return
        }

        num = rand.Intn(num)

	content := postCfg.postContent[num]
	postParams := map[string]string{
		"content": content,
		"pid":     fmt.Sprintf("%d", pd.pid),
	}

        /*处理发送图片*/
        var picIndex int
        var picName  string
        var picPath  string
        picnum := len(postCfg.pics)
        if picnum != 0 {
           picIndex = rand.Intn(picnum)
           postParams["pics"] = "[{\"key\":\"photo_0\",\"upload\":\"NO\"}]"
           /*postParams["pics"] = "[{\"key\":\"photo_0\",\"upload\":\"NO\"},{\"key\":\"photo_1\",\"upload\":\"NO\"}]"*/

           picName = "photo_0"
           picPath = postCfg.pics[picIndex]
        }


	if *verbose {
           fmt.Println("模拟发贴: title",pd.title," ",  postParams, picName, picPath, time.Now().String())
           return
        }

	res, err := myhttp.MultipartFormPost(postAddr, postParams, picName, picPath, u.cookieJar)
	if err != nil {
		fmt.Println("err:", err)
                return
	}

	js, err := simplejson.NewJson(res)
	if err != nil {
		fmt.Println("err:", err)
		return
	}

	_, ok := js.CheckGet("ec")
	if ok {
		s, err := js.Get("ec").Int()
		if err != nil {
			fmt.Println("decode json err: get 'ec' failed!")
			return
		}
		if s == 0 {
			em, _ := js.Get("em").String()
			floor, _ := js.Get("data").Get("floor").String()

			u.postNum++
                        fmt.Println(time.Now().String(), "post:", pd.title, "  pid: ", pd.pid,"  tid: ", pd.tid, " ", em," floor: ", floor,  "N=",u.postNum)

                        /*reset*/
                        u.errCount = 0
			return
		} else {
			em, _ := js.Get("em").String()

                        fmt.Printf("pid: %d  post err: %s title:%s, content:%s\n", pd.pid, em, pd.title, content)

                        u.errCount++
                        if u.errCount > 5 {
                           /*下线*/
                           fmt.Println(u.idStr, " post err > 5, offline")
                           momoManagerDel(u.idStr)
                        }

			return
		}
	}

	return
}

/*
	jsdata := []byte(
		`{
    "data": {
        "count": 20,
        "index": 0,
        "posts": [
            {
                "addr": "湖南岳阳",
                "comment_count": 19683,
                "content": "不要小看任何肌肤问题",
                "create_time": 1385600701,
                "del_msg": "",
                "distance": 782290.2,
                "elite": 1,
                "emotion_body": "",
                "emotion_library": "",
                "emotion_name": "",
                "floor": "楼主",
                "hot": 0,
                "new": 0,
                "owner": "81133406",
                "pics": [],
                "pid": "12245946",
                "reply_time": 1397119994,
                "status": 1,
                "tid": "10000187",
                "title": "我是护肤老题大家抛过来。",
                "top": 1,
                "user": {
                    "avatar": "A6CDC5AE-7092-63BC-C942-A3BA3A3A5A4D",
                    "distance": 781920,
                    "momoid": "81133406",
                    "name": "㊣护肤顾问小南"
                }
            },
            {
                "addr": "河北唐山",
                "comment_count": 1355,
                "content": "如帖子请大谢谢合微笑]谨慎哪！",
                "create_time": 1388248056,
                "del_msg": "",
                "distance": 1944189.5,
                "elite": 1,
                "emotion_body": "",
                "emotion_library": "",
                "emotion_name": "",
                "floor": "楼主",
                "hot": 0,
                "new": 0,
                "owner": "38872364",
                "pics": [
                    "515C5AAC-DAE5-0A39-BD46-57393161E51A",
                    "FA6FE684-49FA-0133-548A-F1DACB51D837"
                ],
                "pid": "13767798",
                "reply_time": 1397119181,
                "status": 1,
                "tid": "10000187",
                "title": "�【不要在问我为什么会禁言】",
                "top": 1,
                "user": {
                    "avatar": "8CA08EAF-07C6-BCBB-D489-72269F6790EC",
                    "distance": -2,
                    "momoid": "38872364",
                    "name": "�_老牛B"
                }
            },
            {
                "addr": "湖州",
                "comment_count": 8,
                "content": "只用了一个月的时间哦，[偷笑][偷笑][偷笑][偷笑]",
                "create_time": 1397117252,
                "del_msg": "",
                "distance": 1095218.7,
                "elite": 0,
                "emotion_body": "",
                "emotion_library": "",
                "emotion_name": "",
                "floor": "楼主",
                "hot": 0,
                "new": 1,
                "owner": "80420612",
                "pics": [
                    "D0B8D376-F57B-30D9-978F-36FB8B4EF13F",
                    "CB824826-265A-17AC-F259-6305A520036D"
                ],
                "pid": "19150151",
                "reply_time": 1397120499,
                "status": 1,
                "tid": "10000187",
                "title": "看我的胸变大了么",
                "top": 0,
                "user": {
                    "avatar": "A6EC4656-4974-33FC-C310-67C22FEB3D74",
                    "distance": 1095351,
                    "momoid": "80420612",
                    "name": "岛岛"
                }
            }
        ],
        "remain": 1,
        "total": 114779
    },
    "ec": 0,
    "em": "success",
    "timesec": 1397120500
}
`)
*/

func (u *User) TiebaPostLoopScan() {

	randNum := rand.Intn(4000)

	for {
		select {
		case <-u.exitChan:
			goto finish

		case <-time.After(postCfg.scanTimeLimit*time.Second + time.Duration(randNum)*time.Millisecond):
			u.TiebaPostScan()
		}

	}

finish:
}

func (u *User) TiebaPostScan() {

	for i, tid := range postCfg.tiebaId {

		fmt.Printf("ScanTieba: %d, %s\n", i, tid)

	if u.session == ""{
           fmt.Printf("Err: %d session nil\n", u.id)
	   return
	}

	var tiebaListAddr = "https://api.immomo.com/api/tieba/post/lists?fr=" + fmt.Sprintf("%d", u.id)

	loginParams := map[string]string{
		"count": "20",
		"index": "0",
		"sort":  "1",
			"tid":   tid,
	}

	jsdata, err := myhttp.MultipartFormPost(tiebaListAddr, loginParams, "", "", u.cookieJar)
	if err != nil {
		fmt.Println("err:", err)
                return
	}

	js, err := simplejson.NewJson(jsdata)
	if err != nil {
		fmt.Println("err:", err)
		return
	}

	/*fmt.Println("解析数据ing")*/
	_, ok := js.CheckGet("ec")
	if ok {
		ec, err := js.Get("ec").Int()
		if err != nil {
			fmt.Println("decode TiebaPost json err: get 'ec' failed!")
			return
		}
		if ec == 0 {

			timesec, _ := js.Get("timesec").Int()

			posts := js.Get("data").GetPath("posts")
			ps, _ := js.Get("data").GetPath("posts").Array()
			for i, _ := range ps {

				comment_count, _ := posts.GetIndex(i).Get("comment_count").Int()
				create_time, _ := posts.GetIndex(i).Get("create_time").Int()

				tidString, _ := posts.GetIndex(i).Get("tid").String()
				tid, _ := strconv.Atoi(tidString)

				pidString, _ := posts.GetIndex(i).Get("pid").String()
				pid, _ := strconv.Atoi(pidString)

				title, _ := posts.GetIndex(i).Get("title").String()

				if _, ok := u.postPidMap[pid]; ok {
					/*fmt.Println("test 旧贴 map", ok, pid, v)*/

				} else {
					u.postPidMap[pid] = 1

					if create_time+60*postCfg.postTimeLimit > timesec && comment_count <= postCfg.comment_count {

						postdata := &tiebaPostData{
							tid:           tid,
							pid:           pid,
							comment_count: comment_count,
							title:         title,
						}

						/*fmt.Println("新发贴目标:  ", postdata)*/
						u.postDataChan <- postdata

					} else {
						/*fmt.Println("不符合规则:  ", title)*/
					}
				}

			}

				/*return*/
		} else {
			em, _ := js.Get("em").String()

			fmt.Println("get teibaPost data err: ", em)

			return
		}
	} else {
		fmt.Println("get ec failed!")
	}
	}

}
func (u *User) Welcome(cookieJar http.CookieJar) {
	var welcomeAddr = "https://api.immomo.com/api/welcomeconfig/android?version=387963&fr=" + fmt.Sprintf("%d", u.id)

	/*var welcomeAddr = "http://api.immomo.com"*/

	response := myhttp.Get(welcomeAddr, cookieJar)

	if true {
		/*defer response.Body.Close()*/
		contents, err := ioutil.ReadAll(bytes.NewReader(response))
		if err != nil {
			fmt.Printf("%s\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s\n", string(contents))
		/*err = ioutil.WriteFile("output.html", contents, 0644)*/
		/*if err != nil { panic(err) }*/
		return

		/*s := string(contents)*/

		/*doc, err := html.Parse(strings.NewReader(s))*/
		if err != nil {
			// ...
			fmt.Println("error")
		}
		var f func(*html.Node) string
		f = func(n *html.Node) (hash string) {
			/*fmt.Println(n)*/
			if n.Type == html.ElementNode && n.Data == "input" {
				for _, a := range n.Attr {
					/*fmt.Println(a)*/
					if a.Key == "value" {
						fmt.Println(a.Val, len(a.Val))
						/*break*/
						hash = a.Val
						return
					}
				}
			}
			if n.Type == html.TextNode {
				/*fmt.Println("text node:", n.Data)*/
			}

			for c := n.FirstChild; c != nil; c = c.NextSibling {
				hash = f(c)
				if hash != "" {
					break
				}
			}

			return hash
		}

		/*formhash = f(doc)*/

	}

	/*resource := "/logging.php?action=login"*/

	h := md5.New()
	h.Write([]byte("lr05"))
	passwd := hex.EncodeToString(h.Sum(nil))
	fmt.Printf("md5=%s\n", passwd)

	data := url.Values{}
	/*data.Set("formhash", formhash)*/
	data.Add("referer", "index.php")
	data.Add("loginfield", "username")
	data.Add("username", "jskuangren") //修改
	data.Add("password", passwd)
	data.Add("questionid", "0")
	data.Add("answer", "")
	data.Add("cookietime", "315360000")
	data.Add("loginmode", "")
	data.Add("styleid", "")
	/*gb2312String,_ := iconv.ConvertString("%CC%E1+%26%23160%3B+%BD%BB", "utf-8", "gb2312")*/
	gb2312String, _ := iconv.ConvertString("提 &#160; 交", "utf-8", "gb2312")
	fmt.Println(gb2312String)
	data.Add("loginsubmit", gb2312String)

	/*u, _ := url.ParseRequestURI(loginAddr)*/
	/*u.Path = resource*/
	/*urlStr := fmt.Sprintf("%v", u) // "https://api.com/user/"*/

	var loginAddr string
	myhttp.Post(loginAddr, cookieJar, data.Encode()) // <-- URL-encoded payload

	/*_ = myhttp.Get(loginAddr, cookieJar)*/

	select {}
}
