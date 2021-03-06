package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"github.com/ziutek/mymysql/mysql"
	_ "github.com/ziutek/mymysql/native" // Native engine
	"github.com/ziutek/mymysql/thrsafe"
	"io"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	/*"strconv"*/
	"strings"
	"sync"
	"time"
	/*"sync/atomic"*/
	"code.google.com/p/gcfg"   //config
	"github.com/howeyc/gopass" //password
)

func printOK() {
	fmt.Println("OK")
}

func checkError(err error) {
	if err != nil {
		fmt.Println(err)
		/*panic("error")*/
		os.Exit(1)
	}
}

func checkedResult(rows []mysql.Row, res mysql.Result, err error) ([]mysql.Row, mysql.Result) {
	checkError(err)
	return rows, res
}


type ClientError string

func (e ClientError) Error() string {
	return string(e)
}

const (
	MYSQL_TYPE_DECIMAL = iota // == 0
	MYSQL_TYPE_TINY
	MYSQL_TYPE_SHORT
	MYSQL_TYPE_LONG
	MYSQL_TYPE_FLOAT
	MYSQL_TYPE_DOUBLE
	MYSQL_TYPE_NULL
	MYSQL_TYPE_TIMESTAMP
	MYSQL_TYPE_LONGLONG
	MYSQL_TYPE_INT24
	MYSQL_TYPE_DATE
	MYSQL_TYPE_TIME
	MYSQL_TYPE_DATETIME
	MYSQL_TYPE_YEAR
	MYSQL_TYPE_NEWDATE
	MYSQL_TYPE_VARCHAR
	MYSQL_TYPE_BIT
	MYSQL_TYPE_NEWDECIMAL  = 246
	MYSQL_TYPE_ENUM        = 247
	MYSQL_TYPE_SET         = 248
	MYSQL_TYPE_TINY_BLOB   = 249
	MYSQL_TYPE_MEDIUM_BLOB = 250
	MYSQL_TYPE_LONG_BLOB   = 251
	MYSQL_TYPE_BLOB        = 252
	MYSQL_TYPE_VAR_STRING  = 253
	MYSQL_TYPE_STRING      = 254
	MYSQL_TYPE_GEOMETRY    = 255
)

/* For backward compatibility */
const FIELD_TYPE_DECIMAL = MYSQL_TYPE_DECIMAL
const FIELD_TYPE_NEWDECIMAL = MYSQL_TYPE_NEWDECIMAL
const FIELD_TYPE_TINY = MYSQL_TYPE_TINY
const FIELD_TYPE_SHORT = MYSQL_TYPE_SHORT
const FIELD_TYPE_LONG = MYSQL_TYPE_LONG
const FIELD_TYPE_FLOAT = MYSQL_TYPE_FLOAT
const FIELD_TYPE_DOUBLE = MYSQL_TYPE_DOUBLE
const FIELD_TYPE_NULL = MYSQL_TYPE_NULL
const FIELD_TYPE_TIMESTAMP = MYSQL_TYPE_TIMESTAMP
const FIELD_TYPE_LONGLONG = MYSQL_TYPE_LONGLONG
const FIELD_TYPE_INT24 = MYSQL_TYPE_INT24
const FIELD_TYPE_DATE = MYSQL_TYPE_DATE
const FIELD_TYPE_TIME = MYSQL_TYPE_TIME
const FIELD_TYPE_DATETIME = MYSQL_TYPE_DATETIME
const FIELD_TYPE_YEAR = MYSQL_TYPE_YEAR
const FIELD_TYPE_NEWDATE = MYSQL_TYPE_NEWDATE
const FIELD_TYPE_ENUM = MYSQL_TYPE_ENUM
const FIELD_TYPE_SET = MYSQL_TYPE_SET
const FIELD_TYPE_TINY_BLOB = MYSQL_TYPE_TINY_BLOB
const FIELD_TYPE_MEDIUM_BLOB = MYSQL_TYPE_MEDIUM_BLOB
const FIELD_TYPE_LONG_BLOB = MYSQL_TYPE_LONG_BLOB
const FIELD_TYPE_BLOB = MYSQL_TYPE_BLOB
const FIELD_TYPE_VAR_STRING = MYSQL_TYPE_VAR_STRING
const FIELD_TYPE_STRING = MYSQL_TYPE_STRING
const FIELD_TYPE_CHAR = MYSQL_TYPE_TINY
const FIELD_TYPE_INTERVAL = MYSQL_TYPE_ENUM
const FIELD_TYPE_GEOMETRY = MYSQL_TYPE_GEOMETRY
const FIELD_TYPE_BIT = MYSQL_TYPE_BIT

func addSlashes(str string) string {

	str = strings.Replace(str, "\\", "\\\\", -1)
	str = strings.Replace(str, "'", "\\'", -1)
	str = strings.Replace(str, "\"", "\\\"", -1)

	return str
}

func getMysqlFieldValue(row mysql.Row, fieldName string, fieldIndex int, fieldType byte) string {

	switch fieldType {

	case FIELD_TYPE_LONG, FIELD_TYPE_TINY, FIELD_TYPE_SHORT:
		val := row.Int(fieldIndex)
		str := fmt.Sprintf("%d", val)
		if *verbose {
			fmt.Printf("%20s: %s\n", fieldName, str)
		}
		return str

	case FIELD_TYPE_LONGLONG:
		vallong := row.Int64(fieldIndex)
		str := fmt.Sprintf("%d", vallong)
		if *verbose {
			fmt.Printf("%20s: %s\n", fieldName, str)
		}
		return str

	case FIELD_TYPE_VAR_STRING, MYSQL_TYPE_STRING:
		str := row.Str(fieldIndex)
		str = addSlashes(str)
		if *verbose {
			fmt.Printf("%20s: %s\n", fieldName, str)
		}
		return "\"" + str + "\""

	default:
		fmt.Println("err:", fieldName, fieldIndex, fieldType)
	}

	return ""
}

type mysqlTableData struct {
	insertSqlPrefix string
	prefixIniflag   byte

	sel     mysql.Stmt
	table   string
	counter int32

	replaceField string
	autoIncrementField string
}

type Worker struct {
	workerId int
	srcDb    mysql.Conn
	dstDb    mysql.Conn

	startTime  time.Time
	tableSlice []mysqlTableData
}

func (wk *Worker) DbPrepare(sqlSelect string, sqlFrom string, sqlInner string, sqlWhere string, autoIncrementField string, replaceField string) error {

	sql := "select " + sqlSelect + " from " + "`" + sqlFrom + "`" + sqlInner + " where " + sqlWhere

	if *verbose {
		fmt.Println("DbPrepare sql:", sql)
	}

	sel, err := wk.srcDb.Prepare(sql)

	if err == nil {
		tableData := new(mysqlTableData)

		tableData.sel = sel
		tableData.table = sqlFrom
		tableData.prefixIniflag = 0
		tableData.replaceField = replaceField
		tableData.autoIncrementField = autoIncrementField

		wk.tableSlice = append(wk.tableSlice, *tableData)
	} else {
		/*return  ClientError("authentication error")*/
		fmt.Println(sql)
		return errors.New("DbPrepare: authentication error")
	}

	return nil
}

func (wk *Worker) ProcessMysqlStmt(userdata userData) error {

	/*for _, tableData := range wk.tableSlice {*/
	/*fmt.Println(len(wk.tableSlice), tableData)*/
	/*}*/
	for i, tableData := range wk.tableSlice {
		/*fmt.Println(tableData)*/

		rows, res := checkedResult(tableData.sel.Exec(userdata.uid))

		var fieldIndex int
		var fieldValueStr string
		var sqlPrefix string
		var sql string

		/*fmt.Println("process table id: ", ii)*/
		if tableData.insertSqlPrefix == "" {

			sqlPrefix = "insert into " + tableData.table + " ("

			/*fill the field name*/
			for _, field := range res.Fields() {

                                if field.Name == tableData.autoIncrementField {
                                   /*自动生成新的id*/
                                   continue
                                }
				if fieldIndex == 0 {
					sqlPrefix += "`" + field.Name + "`"
				} else {
					sqlPrefix += ",`" + field.Name + "`"
				}

				fieldIndex++

				/*字段，类型*/
				//fmt.Print("field:  ", field.Name, "\ttype:", field.Type, "\n")
			}

			sqlPrefix += ") "

			tableData.insertSqlPrefix = sqlPrefix
			wk.tableSlice[i].insertSqlPrefix = sqlPrefix
		} else {
			sqlPrefix = tableData.insertSqlPrefix
		}

		sqlPrefix += "values("

		for _, row := range rows {

			fieldIndex = 0
			sql = sqlPrefix

                        fieldValueIndex := 0

			/*fill the values*/
			for _, field := range res.Fields() {

                                if field.Name == tableData.autoIncrementField {
                                   /*自动生成新的id*/

                                   /*replaceField := res.Map(tableData.autoIncrementField)*/
                                   /*uid := row.Int(replaceField)*/
                                   /*fmt.Println("old uid=", uid)*/


                                   fieldIndex++
                                   continue
                                }

                                /*替换原uid为新的uid*/
                                if tableData.replaceField == field.Name {
                                   fieldValueStr = fmt.Sprintf("%d", userdata.newUid)
                                }else{
                                   fieldValueStr = getMysqlFieldValue(row, field.Name, fieldIndex, field.Type)
                                }

				if fieldValueIndex == 0 {
					sql += fieldValueStr
				} else {
					sql += "," + fieldValueStr
				}

				fieldIndex++
                                fieldValueIndex++
			}

			sql += ")"

			if *verbose {
				fmt.Println(sql)
			}

			/*插入*/
                        _, err := wk.dstDb.Start(sql)

                        if err != nil {
                                fmt.Println(sql)
                                panic(err)
                        }

			wk.tableSlice[i].counter++
		}

		/*fmt.Println("slice range process finish",playerUid )*/
	}

	return nil

}

func FlushMysqlCache(tcpCon *tcpConnect) error {
	srcDb := thrsafe.New(tcpCon.srcProto, "", tcpCon.srcAddr, tcpCon.srcUser, tcpCon.srcPass, tcpCon.srcDbname)
	/*更新源数据库的user实时数据*/
	dstDb := thrsafe.New(tcpCon.srcProto, "", tcpCon.srcAddr, tcpCon.srcUser, tcpCon.srcPass, tcpCon.srcDbname)

	fmt.Printf("FlushMysqlCache: Connect to srcDb:%s:%s...\n", tcpCon.srcProto, tcpCon.srcAddr)
	fmt.Printf("FlushMysqlCache: Connect to dstDb:%s:%s...\n", tcpCon.srcProto, tcpCon.srcAddr)

	checkError(srcDb.Connect())
	checkError(dstDb.Connect())

	defer srcDb.Close()
	defer dstDb.Close()

	update, err := dstDb.Prepare("update `texas_user` set `experience`=?,`win`=?,`lose`=?,`discard`=?,`gamemoney`=?,`gamegold`=? where `id`=?")
	checkError(err)

	var sql string
	//sqlPrefix := "update `texas_user` set "

	for i := 0; i < 10; i++ {
		sql = fmt.Sprintf("select * from `texas_user_%d`", i)
		fmt.Println("Process: ", sql)
		res, err := srcDb.Start(sql)
		checkError(err)

		row := res.MakeRow()

		uid := res.Map("uid")
		experience := res.Map("experience")
		discard := res.Map("discard")
		win := res.Map("win")
		lose := res.Map("lose")
		gamemoney := res.Map("gamemoney")
		gamegold := res.Map("gamegold")

		for {
			err := res.ScanRow(row)
			if err == io.EOF {
				// No more rows
				break
			}

			checkError(err)

			/*updateSql := sqlPrefix*/
			/*updateSql += "`experience`=" + strconv.Itoa(row.Int(experience))*/
			/*updateSql += ",`win`=" + strconv.Itoa(row.Int(win))*/
			/*updateSql += ",`lose`=" + strconv.Itoa(row.Int(lose))*/
			/*updateSql += ",`discard`=" + strconv.Itoa(row.Int(discard))*/
			/*updateSql += ",`gamemoney`=" + strconv.FormatInt(row.Int64(gamemoney), 10)*/
			/*updateSql += ",`gamegold`=" + strconv.FormatInt(row.Int64(gamegold), 10)*/
			/*updateSql += " where `id`=" + strconv.Itoa(row.Int(uid))*/

			/*_, err = dstDb.Start(updateSql)*/
			//fmt.Println(updateSql)

			_, _, err = update.Exec(row.Int(experience), row.Int(win), row.Int(lose), row.Int(discard), row.Int64(gamemoney), row.Int64(gamegold), row.Int(uid))

			checkError(err)
		}

	}
	return nil
}

func PrecessChampionship(tcpCon *tcpConnect) (int, int) {
	srcDb := thrsafe.New(tcpCon.srcProto, "", tcpCon.srcAddr, tcpCon.srcUser, tcpCon.srcPass, tcpCon.srcDbname)
	/*更新源数据库的user实时数据*/
	dstDb := thrsafe.New(tcpCon.dstProto, "", tcpCon.dstAddr, tcpCon.dstUser, tcpCon.dstPass, tcpCon.dstDbname)

	fmt.Printf("PrecessChampionship: Connect to srcDb:%s:%s...\n", tcpCon.srcProto, tcpCon.srcAddr)
	fmt.Printf("PrecessChampionship: Connect to dstDb:%s:%s...\n", tcpCon.srcProto, tcpCon.srcAddr)

	checkError(srcDb.Connect())
	checkError(dstDb.Connect())

	defer srcDb.Close()
	defer dstDb.Close()

	/*update, err := dstDb.Prepare("insert into `texas_championship` set `id`=?,`type`=?,`time`=?,`starttime`=?,`endtime`=?,`Bonus`=?,`config_id`=?")*/
	insert, err := dstDb.Prepare("insert into `texas_championship` values (?,?,?,?,?,?,?)")
	checkError(err)

	var sql string

	tm := time.Now().Unix()
	minId := 1 << 16

	sql = fmt.Sprintf("select * from `texas_championship` where `starttime` > %d", tm)
	fmt.Println("Process: ", sql)
	res, err := srcDb.Start(sql)
	checkError(err)

	row := res.MakeRow()

	champId := res.Map("id")
	champType := res.Map("type")
	champTime := res.Map("time")
	champStarttime := res.Map("starttime")
	champEndtime := res.Map("endtime")
	champBonus := res.Map("Bonus")
	champConfig_id := res.Map("config_id")

        rowCount := 0
	for {
		err := res.ScanRow(row)
		if err == io.EOF {
			// No more rows
			break
		}

                rowCount++

		checkError(err)

		if minId > row.Int(champId) {
			minId = row.Int(champId)
		}
		_, _, err = insert.Exec(row.Int(champId), row.Int(champType), row.Int(champTime), row.Int(champStarttime), row.Int(champEndtime), row.Int(champBonus), row.Int(champConfig_id))

		checkError(err)
	}

	return minId, rowCount
}


var texasUserInsertSqlPrefix = ""
func insertNewAndGetUid(userdatas *[]userData, rows []mysql.Row, res mysql.Result, dstDb mysql.Conn) error {

   var fieldIndex int
   var fieldValueStr string
   var sqlPrefix string
   var sql string

   table := "texas_user"
   replaceFieldName := "id"

   if texasUserInsertSqlPrefix == "" {

      sqlPrefix = "insert into " + table + " ("

      /*fill the field name*/
      for _, field := range res.Fields() {

         if field.Name == replaceFieldName {
            /*自动生成新的id*/
            continue
         }

         if fieldIndex == 0 {
            sqlPrefix += "`" + field.Name + "`"
         } else {
            sqlPrefix += ",`" + field.Name + "`"
         }

         fieldIndex++

         /*字段，类型*/
         //fmt.Print("field:  ", field.Name, "\ttype:", field.Type, "\n")
      }

      sqlPrefix += ") "

      texasUserInsertSqlPrefix = sqlPrefix
   } else {
      sqlPrefix = texasUserInsertSqlPrefix
   }

   sqlPrefix += "values("

   for _, row := range rows {

      fieldIndex = 0
      sql = sqlPrefix
      fieldValueIndex := 0

      userdata := userData{}
      /*fill the values*/
      for _, field := range res.Fields() {

         if field.Name == replaceFieldName {
            /*自动生成新的id*/

            replaceField := res.Map(replaceFieldName)
            uid := row.Int(replaceField)

            userdata.uid = uid
            /*fieldValueStr = getMysqlFieldValue(row, field.Name, fieldIndex, field.Type)*/
            /*fmt.Println("old uid=", fieldValueStr)*/
            /*fmt.Println("old uid=", uid)*/
            fieldIndex++
            continue
         }

         fieldValueStr = getMysqlFieldValue(row, field.Name, fieldIndex, field.Type)

         if fieldValueIndex == 0 {
            sql += fieldValueStr
         } else {
            sql += "," + fieldValueStr
         }

         fieldIndex++
         fieldValueIndex++
      }

      sql += ")"

      if *verbose {
         fmt.Println(sql)
      }

      /*插入*/
      dRes, err := dstDb.Start(sql)

      if err != nil {
         fmt.Println(sql)
         panic(err)
      }


      newUid := dRes.InsertId()

      userdata.newUid = int(newUid)
      /*fmt.Println("new uid=", newUid)*/
      /**userdatas = append(*userdatas, userdata)*/
      *userdatas = append(*userdatas, userdata)


   }

   return nil
}


func getUids(userdatas *[]userData, sel mysql.Stmt, startUid int, dstDb mysql.Conn) error {

	rows, res := checkedResult(sel.Exec(startUid))

        insertNewAndGetUid(userdatas, rows, res, dstDb)

	/*id := res.Map("id")*/

	/*for _, row := range rows {*/

                /*uid := row.Int(id)*/
                /**uids = append(*uids, uid)*/

                /*[>insertNewAndGetUid(rows, res, dstDb)<]*/
	/*}*/

	return nil
}

type tcpConnect struct {
	srcUser   string
	srcPass   string
	srcDbname string
	srcProto  string
	srcAddr   string

	dstUser   string
	dstPass   string
	dstDbname string
	dstProto  string
	dstAddr   string

	appId int
}

type statistics struct {
	mapdata map[string]int32
	sync.Mutex
}

type userData struct {
   uid     int
   newUid  int
}

func newWorker(workerId int) *Worker {
	n := &Worker{
		workerId:   workerId,
		tableSlice: []mysqlTableData{},
	}

	return n
}

func getUidWorker(tcpCon *tcpConnect, userChan chan userData, exitChan chan int) error {

	srcDb := thrsafe.New(tcpCon.srcProto, "", tcpCon.srcAddr, tcpCon.srcUser, tcpCon.srcPass, tcpCon.srcDbname)
        dstDb := thrsafe.New(tcpCon.dstProto, "", tcpCon.dstAddr, tcpCon.dstUser, tcpCon.dstPass, tcpCon.dstDbname)

	fmt.Printf("getUidWorker Connect to srcDb:%s:%s...\n", tcpCon.srcProto, tcpCon.srcAddr)
        fmt.Printf("getUidWorker Connect to dstDb:%s:%s...\n", tcpCon.dstProto, tcpCon.dstAddr)

	checkError(srcDb.Connect())
        checkError(dstDb.Connect())
	defer srcDb.Close()
        defer dstDb.Close()

	fmt.Println("getUidWorker 初始化完成")
	/*fmt.Printf("srcDb: %#v\n", srcDb)*/
	/*fmt.Printf("dstDb: %#v\n", dstDb)*/

	start := time.Now()

	userDatas := make([]userData, 0, 100)

	startUid := 0

	/*sel, err := srcDb.Prepare(fmt.Sprintf("select * from `texas_user` where `id` > ? and `appid`=%d order by `id` limit 100", tcpCon.appId))*/
	sel, err := srcDb.Prepare(fmt.Sprintf("select * from `texas_user` where `id` > ?  order by `id` limit 100"))
	checkError(err)

	row, _, err := srcDb.QueryFirst(fmt.Sprintf("select count(*) as `total` from `texas_user`"))
	checkError(err)

	totalUid := row.Int(0)
	fmt.Printf("total uids = %d\n", totalUid)

	var currentUid float64
	percentage := 0

	for {
		getUids(&userDatas, sel, startUid, dstDb)

		/*fmt.Printf("print uids\n")*/

		var userdata userData
		for _, userdata = range userDatas {

			if *verbose {
				fmt.Printf("put uid=%d into userChan\n", userdata)
			}

			startUid = userdata.uid

			userChan <- userdata

			currentUid++

			/*process percentage*/
			if tmpPercentage := currentUid / float64(totalUid) * 100; int(tmpPercentage) != percentage {

				elapsedTime := time.Since(start)

				percentage = int(tmpPercentage)

				fmt.Printf("Progress: %2d%%  elapsed time: %s\n", percentage, elapsedTime)
			}
		}

		userDatas = []userData{}

		if startUid != userdata.uid {
			startUid = userdata.uid
		}

		if userdata.uid == 0 {
			goto exit
		}

		/*fmt.Println("test exit")*/
		/*goto exit*/
	}

exit:
	if len(userChan) != 0 {
		time.Sleep(time.Millisecond * 100)
		fmt.Println("progress exiting: wait userChan read finish")
		goto exit
	}

	fmt.Println("progress exit")

	close(exitChan)

	return nil

}

/*func pubWorker(tcpCon *tcpConnect, uidChan chan int, exitChan chan int, workerId int, startTime time.Time, stat *statistics, champId int) error {*/
func pubWorker(tcpCon *tcpConnect, userChan chan userData, exitChan chan int, workerId int, startTime time.Time, stat *statistics) error {

	srcDb := thrsafe.New(tcpCon.srcProto, "", tcpCon.srcAddr, tcpCon.srcUser, tcpCon.srcPass, tcpCon.srcDbname)
	dstDb := thrsafe.New(tcpCon.dstProto, "", tcpCon.dstAddr, tcpCon.dstUser, tcpCon.dstPass, tcpCon.dstDbname)

	fmt.Printf("worker: %d Connect to srcDb:%s:%s...\n", workerId, tcpCon.srcProto, tcpCon.srcAddr)
	fmt.Printf("worker: %d Connect to dstDb:%s:%s...\n", workerId, tcpCon.dstProto, tcpCon.dstAddr)

	checkError(srcDb.Connect())
	checkError(dstDb.Connect())

	fmt.Printf("worker: %d 初始化成功\n", workerId)

	defer srcDb.Close()
	defer dstDb.Close()

        tm := time.Now().Unix()

	worker := newWorker(workerId)

	worker.srcDb = srcDb
	worker.dstDb = dstDb
	worker.startTime = startTime

	/*mysql table process*/

	/*texas_user*/
	/*err := worker.DbPrepare("*", "texas_user", "",  "id = ? limit 1")*/
	/*checkError(err)*/

        /*vip卡*/
        /*err := worker.DbPrepare("texas_user_vip", "texas_user_vip_tmp", "uid = ? and (expiretime = 0 or expiretime > "+strconv.FormatInt(tm, 10)+ ")")*/
        err := worker.DbPrepare("*", "texas_user_vip", "", fmt.Sprintf("uid = ? and (expiretime = 0 or expiretime > %d)", tm), "id", "uid")
        checkError(err)

        /*经验卡*/
        /*err = worker.DbPrepare("texas_user_expcard", "texas_user_expcard_tmp", "uid = ? and (end_time =0 or end_time > "+strconv.FormatInt(tm, 10)+ ")")*/
        err = worker.DbPrepare("*", "texas_user_expcard", "", fmt.Sprintf("uid = ? and (end_time =0 or end_time > %d)", tm), "id", "uid")
        checkError(err)

        /*喇叭*/
        err = worker.DbPrepare("*", "texas_user_loudspeaker", "", "uid = ? and `amount` != 0 ", "id", "uid")
        checkError(err)

        /*饰品*/
        err = worker.DbPrepare("*", "texas_user_props", "", fmt.Sprintf("uid = ? and `expiretime` > %d", tm), "id", "uid")
        checkError(err)

        /*互动表情*/
        err = worker.DbPrepare("*", "texas_user_expression", "", "uid = ? and `amount`!=0", "id", "uid")
        checkError(err)

        /*成就*/
        err = worker.DbPrepare("*", "texas_user_achievement", "", "uid = ?", "", "uid")
        checkError(err)

        /*好友*/
        /*err = worker.DbPrepare("*", "texas_user_friend", "", "uid = ?")*/
        /*checkError(err)*/

        /*荷官设置*/
        /*err = worker.DbPrepare("*", "texas_user_dealer", "", "uid = ?")*/
        /*checkError(err)*/

        /*游戏任务*/
        err = worker.DbPrepare("*", "texas_user_task", "", "uid = ?", "id", "uid")
        checkError(err)

        /*[>新手教程奖励<]*/
        /*err = worker.DbPrepare("*", "texas_user_course_award", "", "uid = ?")*/
        /*checkError(err)*/

        /*[>大家乐记录<]*/
        /*err = worker.DbPrepare("*", "texas_user_dajiale", "", "uid = ?")*/
        /*checkError(err)*/

        /*[>玩家步骤<]*/
        /*err = worker.DbPrepare("*", "texas_user_step", "", "uid = ?")*/
        /*checkError(err)*/

        /*[>chest<]*/
        /*err = worker.DbPrepare("*", "texas_user_chest", "", "uid = ?")*/
        /*checkError(err)*/

        /*[>ban<]*/
        /*err = worker.DbPrepare("*", "texas_user_ban", "", "uid = ?")*/
        /*checkError(err)*/

        /*[>私人房间<]*/
        /*err = worker.DbPrepare("*", "texas_user_room_log", "", "uid = ?")*/
        /*checkError(err)*/

        /*[>幸运大转盘<]*/
        /*err = worker.DbPrepare("*", "texas_user_lucky_wheel", "", "uid = ?")*/
        /*checkError(err)*/

        /*[>筹码变更日志<]*/
        /*[>err = worker.DbPrepare("*", texas_user_money_log", "", fmt.Sprintf("uid = ? and `time` > %d", tm - 30*24*3600 ))<]*/
        /*[>checkError(err)<]*/

        /*[>锦标赛报名<]*/
        /*[>err = worker.DbPrepare("*", "texas_championship_registration", "", fmt.Sprintf("`uid`=? and `championship_id` > %d", champId))<]*/
        /*[>checkError(err)<]*/

        /*[>order 表<]*/
        /*err = worker.DbPrepare("*", "texas_order", "", "`uid`=?")*/
        /*checkError(err)*/

        /*err = worker.DbPrepare("texas_order_facebook.*", "texas_order_facebook", "inner join `texas_order` on `orderid`=`texas_order`.`id`", "`texas_order`.`uid`=?")*/
        /*checkError(err)*/

        /*err = worker.DbPrepare("texas_order_apple.*", "texas_order_apple", "inner join `texas_order` on `orderid`=`texas_order`.`id`", "`texas_order`.`uid`=?")*/
        /*checkError(err)*/

        /*err = worker.DbPrepare("texas_order_google.*", "texas_order_google", "inner join `texas_order` on `orderid`=`texas_order`.`id`", "`texas_order`.`uid`=?")*/
        /*checkError(err)*/

        /*err = worker.DbPrepare("*", "texas_order_google_log", "", "`uid`=?")*/
        /*checkError(err)*/

        /*err = worker.DbPrepare("texas_order_facebook_log.*", "texas_order_facebook_log",*/
        /*"inner join `texas_order_facebook`" +*/
        /*"on `texas_order_facebook_log`.`facebook_order`=`texas_order_facebook`.`orderid`" +*/
        /*" inner join `texas_order` on `texas_order`.`id`=`texas_order_facebook`.`orderid`", "`texas_order`.`uid`=?")*/
        /*checkError(err)*/

        /*err = worker.DbPrepare("texas_order_facebook_log.*", "texas_order_facebook_log", "inner join `texas_order_facebook` on `texas_order_facebook_log`.`facebook_order`=`texas_order_facebook`.`orderid` inner join `texas_order` on `texas_order`.`id`=`texas_order_facebook`.`orderid`", "`texas_order`.`uid`=?")*/
        /*checkError(err)*/


	/*loop:*/
	for {

		select {
		case userdata, ok := <-userChan:
			if ok {
				if *verbose {
					fmt.Println("process user:", userdata)
				}
                                /*start := time.Now()*/
                                worker.ProcessMysqlStmt(userdata)
                                /*useTime := time.Since(start)*/
			} else {
				fmt.Printf("worker: %d userChan close\n", workerId)
				goto exitTab
			}

		case <-exitChan:
			/*break Loop*/ //与goto 效果一致
			goto exitTab
		}
	}

exitTab:
	fmt.Printf("worker: %d exit\n", workerId)
	for _, tableData := range worker.tableSlice {
		stat.Lock()
		stat.mapdata[tableData.table] += tableData.counter
		stat.Unlock()
	}
	return nil
}

func generalConfigFile() {
	b := []byte(
		`[App]
AppID=1

[SrcDB]
Name=texas
User=texas
Ip=172.16.5.200
Port=3306
Pwd=

[DstDB]
Name=texas_hungary
User=texas
Ip=172.16.5.200
Port=3306
Pwd=
`)
	err := ioutil.WriteFile("texas_mysql.ini", b, 0644)
	if err != nil {
		log.Fatalf("Failed to general texas_mysql.ini: %s", err)
		return
	}
	fmt.Println("general texas_mysql.ini successful")
}

var (
	// basic options
	/*showVersion = flag.Bool("version", false, "print version string")*/
	verbose          = flag.Bool("verbose", false, "enable verbose logging")
	outPutConfigFile = flag.Bool("o", false, "generate the texas_mysql.ini file")
)

type Config struct {
	App struct {
		AppID int
	}

	SrcDB struct {
		Name string
		User string
		Ip   string
		Port string
		Pwd  string
	}

	DstDB struct {
		Name string
		User string
		Ip   string
		Port string
		Pwd  string
	}
}

func main() {

	var cfg Config
	var dstDbPwd string
	var srcDbPwd string

	flag.Parse()
	if *outPutConfigFile {
		generalConfigFile()
		return
	}

	err := gcfg.ReadFileInto(&cfg, "texas_mysql.ini")
	if err != nil {
		log.Fatalf("Failed to parse texas_mysql.ini: %s", err)
	}

	fmt.Println("appid: ", cfg.App.AppID)

	fmt.Println("SrcDB Name:", cfg.SrcDB.Name)
	fmt.Println("SrcDB User:", cfg.SrcDB.User)
	fmt.Println("SrcDB Ip:", cfg.SrcDB.Ip)
	fmt.Println("SrcDB Port:", cfg.SrcDB.Port)
	fmt.Println("SrcDB Pwd:", cfg.SrcDB.Pwd)

	fmt.Println("DstDB Name:", cfg.DstDB.Name)
	fmt.Println("DstDB Port:", cfg.DstDB.Port)
	fmt.Println("DstDB Ip:", cfg.DstDB.Ip)
	fmt.Println("DstDB User:", cfg.DstDB.User)
	fmt.Println("DstDB Pwd:", cfg.DstDB.Pwd)

	if cfg.App.AppID == 0 {
		log.Fatalf("config: AppID err")
	}

        if cfg.SrcDB.Name == "" {
                log.Fatalf("config: SrcDB Name err")
        }

	if cfg.SrcDB.Ip == "" {
		log.Fatalf("config: SrcDB Ip err")
	}

	/*if cfg.SrcDB.User == "" {*/
		/*log.Fatalf("config: SrcDB User err")*/
	/*}*/

	if cfg.DstDB.Name == "" {
		log.Fatalf("config: DstDB Name err")
	}

	if cfg.DstDB.Ip == "" {
		log.Fatalf("config: DstDB Ip err")
	}

        /*if cfg.DstDB.User == "" {*/
                /*log.Fatalf("config: DstDB User err")*/
        /*}*/

	if cfg.SrcDB.Pwd == "" {
		fmt.Println("please input the SrcDB password")
		srcDbPwd = string(gopass.GetPasswd())
	} else {
		srcDbPwd = cfg.SrcDB.Pwd
	}

	if cfg.DstDB.Pwd == "" {
		fmt.Println("please input the DstDB password")
		dstDbPwd = string(gopass.GetPasswd())
	} else {
		dstDbPwd = cfg.DstDB.Pwd
	}

	tcpCon := new(tcpConnect)
	tcpCon.srcUser = cfg.SrcDB.User
	tcpCon.srcPass = srcDbPwd
	tcpCon.srcDbname = cfg.SrcDB.Name
	tcpCon.srcProto = "tcp"
	tcpCon.srcAddr = cfg.SrcDB.Ip + ":" + cfg.SrcDB.Port

	tcpCon.dstUser = cfg.DstDB.User
	tcpCon.dstPass = dstDbPwd
	tcpCon.dstDbname = cfg.DstDB.Name
	tcpCon.dstProto = "tcp"
	tcpCon.dstAddr = cfg.DstDB.Ip + ":" + cfg.DstDB.Port

	tcpCon.appId = cfg.App.AppID

	/*pause*/
	fmt.Println("press Enter key to continue or Ctrl-C for break")
	bufio.NewReader(os.Stdin).ReadBytes('\n')

	/*start:*/
	cpusNum := runtime.GOMAXPROCS(0)
	cpusNum = 4

	userChan := make(chan userData, cpusNum)
	exitChan := make(chan int)

        if FlushMysqlCache(tcpCon) == nil {
                fmt.Println("flush mysql texas_user_x cache successful.\n\n")
        }

        /*champId, champCount := PrecessChampionship(tcpCon)*/
        /*fmt.Println(fmt.Sprintf("vaild championship champCount=%d minId=%d",champCount, champId))*/
	/*fmt.Println()*/

	/*go getUidWorker(tcpCon, userChan, exitChan)*/
        go getUidWorker(tcpCon, userChan, exitChan)

	var wg sync.WaitGroup
	stat := &statistics{mapdata: make(map[string]int32)}

	start := time.Now()
	for j := 0; j < cpusNum; j++ {
		//for j := 0; j < 4; j++ {
		wg.Add(1)
		go func(id int, startTime time.Time) {
			pubWorker(tcpCon, userChan, exitChan, id, startTime, stat)
			wg.Done()
		}(j, start)
	}

	wg.Wait()
	end := time.Now()
	duration := end.Sub(start)

	/*状态统计*/
	fmt.Println("\nstatistics:")
	fmt.Printf("%-30s    %s\n", "duration", duration)

	for key, value := range stat.mapdata {
		fmt.Printf("%-30s    %5d rows\n", key, value)
	}

	return
}
