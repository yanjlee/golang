package myhttp

import (
   "net/http"
   "io/ioutil"
   "io"
   "compress/gzip"
   "bytes"
   "fmt"
   /*"net/http/cookiejar"*/
   /*"github.com/51nb/myhttp"*/
   "mime/multipart"
   "crypto/tls"
   "os"
   "path/filepath"
)

/*
User-Agent:       MomoChat/4.10 Android/117 (SPH-L720; Android 4.2.2; Gapps 1; en_US; 1)
Connection:       Keep-Alive
Charset:          UTF-8
Content-Type:     multipart/form-data; boundary=---------------------------7da2137580612
Content-Length:   0
Accept-Language:  zh-CN
Host:             referee.immomo.com
Accept-Encoding:  gzip
*/

func Get(url string,cookie http.CookieJar) []byte{
   tr := &http.Transport{
      TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
   }
   client := &http.Client{
      /*CheckRedirect: nil,*/
      Jar:cookie,
      Transport: tr,
   }
   reqest, _ := http.NewRequest("GET", url, nil)

   reqest.Header.Set("User-Agent","MomoChat/4.10 Android/117 (SPH-L720; Android 4.2.2; Gapps 1; en_US; 1)")
   reqest.Header.Set("Connection","keep-alive")
   reqest.Header.Set("Charset","UTF-8")
   reqest.Header.Set("Content-Type","multipart/form-data; boundary=---------------------------7da2137580612")
   reqest.Header.Set("Accept-Language","zh-CN")
   /*reqest.Header.Set("Host","referee.immomo.com")*/
   reqest.Header.Set("Accept-Encoding","gzip")

   /*reqest.Header.Add("Content-Type", "application/x-www-form-urlencoded")*/


   /*if(len(cookie) > 0){*/
      /*fmt.Println("dealing with cookie:"+cookie)*/
      /*array:=strings.Split(cookie,";")*/
      /*for item:= range array{*/
         /*array2:=strings.Split(array[item],"=")*/
         /*if(len(array2)==2){*/
            /*cookieObj:= http.Cookie{}*/
            /*cookieObj.Name=array2[0]*/
            /*cookieObj.Value=array2[1]*/
            /*reqest.AddCookie(&cookieObj)*/
         /*}else{*/
            /*fmt.Println("error,index out of range:"+array[item])*/
         /*}*/
      /*}*/
   /*}*/

   resp, err := client.Do(reqest)

   if err != nil {
      fmt.Println(url,err)
      return nil
   }

   defer resp.Body.Close()

   var reader io.ReadCloser
   switch resp.Header.Get("Content-Encoding") {
   case "gzip":
      reader, err = gzip.NewReader(resp.Body)
      if err != nil {
         fmt.Println(url,err)
         return nil
      }
      defer reader.Close()
   default:
      reader = resp.Body
   }


   if(reader!=nil){
      body, err := ioutil.ReadAll(reader)
      if err != nil {
         fmt.Println(url,err)
         return nil
      }
      return body

   }
   return nil
}



func httpRedirect(req *http.Request, via []*http.Request) error {

   if req.Method == "GET"{
      /*fmt.Printf("via=%#v \n", via[0])*/
      /*fmt.Printf("reqest=%#v \n", req)*/
      /*fmt.Println("exit redirct")*/

      req.Header = via[0].Header
   }
   return nil
}


var boundary = "---------------------------7da2137580612"

func MultipartFormPost(url string, params map[string]string, paramName string, path string, cookie http.CookieJar) ([]byte, error) {

   tr := &http.Transport{
      TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
   }
   client := &http.Client{
      CheckRedirect: httpRedirect,
      Jar:cookie,
      Transport: tr,
   }

   body := &bytes.Buffer{}
   writer := multipart.NewWriter(body)

   writer.SetBoundary(boundary)

   for key, val := range params {
      _ = writer.WriteField(key, val)
   }

   if path != "" && paramName != "" {

      file, err := os.Open(path)
      if err != nil {
         return nil, err
      }
      defer file.Close()

      part, err := writer.CreateFormFile(paramName, filepath.Base(path))
      if err != nil {
         return nil, err
      }

      _, err = io.Copy(part, file)
      if err != nil {
         return nil, err
      }
   }

   err := writer.Close()
   if err != nil {
      return nil, err
   }

   reqest, _ := http.NewRequest("POST", url, body)


   reqest.Header.Set("User-Agent","MomoChat/4.10 Android/117 (SPH-L720; Android 4.2.2; Gapps 1; en_US; 1)")
   reqest.Header.Set("Connection","keep-alive")
   reqest.Header.Set("Charset","UTF-8")
   reqest.Header.Set("Content-Type","multipart/form-data; boundary="+boundary)
   reqest.Header.Set("Accept-Language","zh-CN")
   reqest.Header.Set("Accept-Encoding","gzip")
   /*reqest.Header.Add("Content-Type", writer.FormDataContentType())*/

   resp, err := client.Do(reqest)

   if err != nil {
      return nil, err
   }

   defer resp.Body.Close()

   var reader io.ReadCloser
   switch resp.Header.Get("Content-Encoding") {
   case "gzip":
      reader, err = gzip.NewReader(resp.Body)
      if err != nil {
         return nil, err
      }
      defer reader.Close()
   default:
      reader = resp.Body
   }

   if(reader!=nil){
      body, err := ioutil.ReadAll(reader)
      if err != nil {
         return nil, err
      }
      return body, nil
   }

   return nil, nil
}



func Post(url string,cookie http.CookieJar,postStr string) []byte{

   fmt.Println("let's post :"+url)

   client := &http.Client{
      CheckRedirect: httpRedirect,
      Jar:cookie,
   }

   postBytesReader := bytes.NewReader([]byte(postStr))
   reqest, _ := http.NewRequest("POST", url, postBytesReader)

   reqest.Header.Set("User-Agent"," Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/31.0.1650.63 Safari/537.36")
   reqest.Header.Set("Accept","text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
   reqest.Header.Set("Accept-Charset","GBK,utf-8;q=0.7,*;q=0.3")
   reqest.Header.Set("Accept-Encoding","gzip,deflate,sdch")
   reqest.Header.Add("Content-Type", "application/x-www-form-urlencoded")
   /*reqest.Header.Add("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")*/
   /*reqest.Header.Set("Accept-Language","zh-CN,zh;q=0.8")*/
   reqest.Header.Set("Accept-Language","en-US,en;q=0.8,zh-CN;q=0.6,zh-TW;q=0.4")
   reqest.Header.Set("Cache-Control","max-age=0")
   reqest.Header.Set("Connection","keep-alive")
   reqest.Header.Set("Referer", url)



   /*if(len(cookie) > 0){*/
      /*fmt.Println("dealing with cookie:"+cookie)*/
      /*array:=strings.Split(cookie,";")*/
      /*for item:= range array{*/
         /*array2:=strings.Split(array[item],"=")*/
         /*if(len(array2)==2){*/
            /*cookieObj:= http.Cookie{}*/
            /*cookieObj.Name=array2[0]*/
            /*cookieObj.Value=array2[1]*/
            /*reqest.AddCookie(&cookieObj)*/
         /*}else{*/
            /*fmt.Println("error,index out of range:"+array[item])*/
         /*}*/
      /*}*/
   /*}*/

   resp, err := client.Do(reqest)

   if err != nil {
      fmt.Println(url,err)
      return nil
   }

   defer resp.Body.Close()

   var reader io.ReadCloser
   switch resp.Header.Get("Content-Encoding") {
   case "gzip":
      reader, err = gzip.NewReader(resp.Body)
      if err != nil {
         fmt.Println(url,err)
         return nil
      }
      defer reader.Close()
   default:
      reader = resp.Body
   }


   if(reader!=nil){
      body, err := ioutil.ReadAll(reader)
      if err != nil {
         fmt.Println(url,err)
         return nil
      }
      return body
   }
   return nil
}
