
package main

import (
   "fmt"
   "sync"
   "syscall"
)



var wg sync.WaitGroup

var UserMap map[string]*User

func momoManagerInt(){
   UserMap = make(map[string]*User)
}


func momoManagerAdd(u *User){

      if _, ok := UserMap[u.idStr]; ok{
         fmt.Println("momoid Duplicate", u.idStr)
         return
      }

      /*fmt.Println(u)*/
      UserMap[u.idStr] = u
}

func momoManagerDel(idStr string){

      if u, ok := UserMap[idStr]; ok{
         u.Exit()
         delete(UserMap, idStr)
         if len(UserMap) == 0 {
            fmt.Println("Process exiting")
            syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
         }
      }else{
         fmt.Println("UserMap err: id not find ", idStr)
      }
}

func momoManagerCheck(idStr string)  bool{

      if _, ok := UserMap[idStr]; ok{

         return true
      }else{
         return false
      }
}

func momoManagerStart(){

        for _, u := range UserMap {
           go u.Action(&wg)
        }
}
func momoManageExit(){

        for _, u := range UserMap {
           u.Exit()
        }
}






