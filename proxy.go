package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"ncm/cfg"
	"net"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

const (
	// fname = "/app/config/setup.toml"
	fname   = "config/setup.toml" // для тестов
	reghtml = 301
)

var tconfig cfg.TomlConfig

var writeData = make(chan string) // канал записи

func set_data(value string) {
	writeData <- value
}

//	func read_data() savedata {
//		return <-readData
//	}

func die(format string, v ...interface{}) {
	os.Stderr.WriteString(fmt.Sprintf(format+"\n", v...))
	os.Exit(1)
}

// Данная функция реализует поток для двунаправленного дампа.
func connection_logger(data chan []byte, conn_n int64, local_info, remote_info string) {
	log_name := fmt.Sprintf("log-%s-%04d-%s-%s.log", format_time(time.Now()),
		conn_n, local_info, remote_info)
	logger_loop(data, log_name)
}

// Данная функция реализует поток логирования. Создает лог-файл и начинает
// принимает сообщения. Каждое сообщение - это кусок данных для помещения
// в лог. Если пришли пустые данные - выходим.
func logger_loop(data chan []byte, log_name string) {
	f, err := os.Create(".//logs//" + log_name)
	if err != nil {
		die("Unable to create file %s, %v\n", log_name, err)
	}
	defer f.Close() // Гарантируем закрытие файла в случае падения.
	for {
		b := <-data
		if len(b) == 0 {
			break
		}
		f.Write(b)
		//		f.Sync() // На всякий случай flush'имся.
	}
}
func format_norm(t time.Time) string {
	return t.Format("2006.01.02 15:04:05")
}
func format_time(t time.Time) string {
	return t.Format("2006.01.02-15.04.05")
}

func printable_addr(a net.Addr) string {
	return strings.Replace(a.String(), ":", "-", -1)
}

// func ndtp_open(b []byte, n int, nd_s *Nd_session) {

// 	data := new(bytes.Buffer)
// 	data.Write(b[:n])
// 	npl := ndtp.Npl{}
// 	_ = binary.Read(data, binary.LittleEndian, &npl)
// 	// if the prefix is wrong - disconnect.
// 	if npl.Signature != ndtp.NPL_PACKET_SIGNATURE {
// 		return
// 	}
// 	nd_s.Event = 0

// 	nph := ndtp.Nph{}
// 	_ = binary.Read(data, binary.LittleEndian, &nph)
// 	if nph.Service_id == ndtp.NPH_SRV_GENERIC_CONTROLS {
// 		if nph.TypeNPH == ndtp.NPH_SGC_CONN_REQUEST {
// 			auth := ndtp.Authorization{}
// 			_ = binary.Read(data, binary.LittleEndian, &auth)

// 			nd_s.Pcount = 1
// 			nd_s.Id_bnst = int32(auth.Peer_address)
// 		}
// 	}

// }

// Структура, в которой передаются параметры соединения. Объединено, чтобы
// не таскать много параметров.

type netCon struct {
	con    net.Conn
	active bool
	// back   bool
}
type ListTo struct {
	from         net.Conn
	Index_head   int
	to           []netCon
	logger       chan []byte
	ack          chan bool
	id_session   int64
	time_session int64
}

type Nd_session struct {
	Event    int
	TimeUnix uint32
	Addr     string
	Id_bnst  int32
	Size     int
	Pcount   int
}

// объекты работы с конектами
// струтктра содержит сведения о коннекте
type con_work struct {
	Id_bnst  int32
	TimeOpen time.Time
	AddrIn   string
	AddrOut  string
	Con      net.Conn
}

var syncon sync.RWMutex
var map_con map[int64]con_work // id connect
var map_bnst map[int32]int64   //id bnst

func init_con(id int64, data con_work) {
	syncon.Lock()
	data.TimeOpen = time.Now()
	map_con[id] = data
	syncon.Unlock()
}
func set_bnst(id int64, id_bnst int32) {
	syncon.Lock()
	ndata := map_con[id]
	ndata.Id_bnst = id_bnst
	map_bnst[id_bnst] = id
	map_con[id] = ndata
	syncon.Unlock()
}
func delete_con(id int64) {
	syncon.Lock()
	ndata := map_con[id]
	delete(map_bnst, ndata.Id_bnst)
	delete(map_con, id)
	syncon.Unlock()
}

// струтктура вывода
type con_web struct {
	Id       int64
	Id_bnst  int32
	TimeOpen string
	AddrIn   string
	AddrOut  string
}

// структура установки параметров
type set_param struct {
	Id         int64
	Id_bnst    int32
	Service_id uint16
	TypeNPH    uint16
	Comstr     string
}

// сортировка вывода

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	type strweb struct {
		Countb  string
		Listcon []con_web
	}
	data := strweb{}
	syncon.RLock()
	for k, d := range map_con {
		data.Listcon = append(data.Listcon, con_web{Id: k, Id_bnst: d.Id_bnst,
			TimeOpen: d.TimeOpen.Format("2006.01.02 15:04:05"), AddrIn: d.AddrIn, AddrOut: d.AddrOut})
	}
	syncon.RUnlock()
	// сортировка вывода
	sort.SliceStable(data.Listcon, func(i, j int) bool {
		return data.Listcon[i].Id_bnst < data.Listcon[j].Id_bnst
	})
	data.Countb = fmt.Sprintf("Кол-во конектов %d", len(data.Listcon))
	tmpl, _ := template.ParseFiles("templates/index.html")
	tmpl.Execute(w, data)

}
func DiscHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ids := vars["id"]
	id, _ := strconv.ParseInt(ids, 10, 64)

	disconnect(id)
	// fmt.Println("del: ", id)
	http.Redirect(w, r, "/", reghtml)
}

func disconnect(id int64) {
	syncon.RLock()
	data := map_con[id]
	con := data.Con
	if con != nil {
		con.Close()
	}
	syncon.RUnlock()
	// return ret
}

// Установка параметров отправки
func SetParamCom(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	fmt.Println(id)
	sp := set_param{}
	sp.Id, _ = strconv.ParseInt(id, 10, 64)
	tmpl, _ := template.ParseFiles("templates/paramcom.html")
	tmpl.Execute(w, sp)
}

// получаем параметры и формируем команы для отправки
func SendCommand(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		fmt.Println(err)
	}
	id := r.FormValue("id")
	Id_bnst := r.FormValue("id_bnst")
	Service_id := r.FormValue("service_id")
	TypeNPH := r.FormValue("typenph")
	Comstr := r.FormValue("comstr")
	quest := r.Form["question"]

	fmt.Printf("Id_bnst=%s, Service_id=%s, TypeNPH = %s Comstr=%s\n id = %s\n",
		Id_bnst, Service_id, TypeNPH, Comstr, id)
	fmt.Println(quest)

	http.Redirect(w, r, "/", reghtml)
}

// Функция, "качающая" данные из одного сокета и передающая их в другой.
func from_to_copy(c *ListTo) {

	nd_s := Nd_session{}
	nd_s.Event = -1
	nd_s.TimeUnix = uint32(time.Now().Unix())
	nd_s.Addr = c.from.LocalAddr().String()
	nd_s.Id_bnst = -1
	nd_s.Size = 0
	nd_s.Pcount = 0
	mdata := con_work{}
	mdata.Con = c.from
	mdata.Id_bnst = -1
	mdata.AddrIn = c.from.LocalAddr().String()
	mdata.AddrOut = c.from.RemoteAddr().String()

	init_con(c.id_session, mdata)
	c.logger <- []byte(fmt.Sprintf("time %s\n  Send  %s  ->  %s \n",
		format_norm(time.Now()),
		c.from.LocalAddr(),
		c.from.RemoteAddr()))

	b := make([]byte, 1024)
	offset := 0
	packet_n := 0
	for {
		n, err := c.from.Read(b)
		if err != nil {
			//	c.logger <- []byte(fmt.Sprintf("Disconnected from %s\n", from_peer))
			break
		}
		offset += n
		packet_n += 1
		if n > 0 {
			if (n > 40) && (nd_s.Event < 0) {
				// ndtp_open(b, n, &nd_s)
				if nd_s.Id_bnst >= 0 {
					set_bnst(c.id_session, nd_s.Id_bnst)
					avNd, err := json.Marshal(nd_s)
					if err == nil {
						c.logger <- avNd
						set_data(fmt.Sprintf("%d,%d,%d,'%s'", 0, c.time_session,
							c.id_session, avNd))
					}
				}
			}
			c.logger <- []byte(fmt.Sprintf("time %s \n send count:%d   from %s   %s\n",
				format_norm(time.Now()), n,
				c.from.LocalAddr(),
				c.from.RemoteAddr()))

			for i, toser := range c.to {
				if toser.active {
					_, err := toser.con.Write(b[:n])
					if err != nil {
						toser.active = false
						// если главный поток
						if i == c.Index_head {
							break
						}
					} else {
						c.logger <- []byte(fmt.Sprintf(" to  %s    %s \n",
							toser.con.LocalAddr(),
							toser.con.RemoteAddr()))
					}
				}
			}
			c.logger <- []byte(hex.Dump(b[:n]))

		}

	}
	c.from.Close()
	for _, toser := range c.to {
		if toser.active {
			toser.con.Close()
		}
	}
	delete_con(c.id_session)
	nd_s.Event = 1
	nd_s.TimeUnix = uint32(time.Now().Unix())
	nd_s.Addr = c.from.LocalAddr().String()
	nd_s.Size = offset
	nd_s.Pcount = packet_n
	avNd, err := json.Marshal(nd_s)
	if err == nil {
		c.logger <- avNd
		set_data(fmt.Sprintf("%d,%d,%d,'%s'", 1, c.time_session,
			c.id_session, avNd))

	}

	c.ack <- true // Посылаем сообщение в главный поток, что мы закончили.
}

// Функция, "качающая" данные из главного сокета и передающая их в другой.
// Попутно ведется логирование.
func pass_through(c *ListTo) {

	from_peer := c.from.RemoteAddr().String() + " -> " + c.from.LocalAddr().String()

	hc := c.to[c.Index_head].con

	b := make([]byte, 1024)
	offset := 0
	packet_n := 0
	for {
		n, err := hc.Read(b)
		if err != nil {
			c.logger <- []byte(fmt.Sprintf("Disconnected time %s\n %s\n",
				format_norm(time.Now()),
				from_peer))
			break
		}
		if n > 0 {
			// Если что-то пришло, то логируем и пересылаем на выход.
			// Это все, что нужно для преобразования в hex-дамп.
			c.logger <- []byte(fmt.Sprintf("Sent (#%d) to %s Dump:\n %s", packet_n, from_peer, hex.Dump(b[:n])))
			c.logger <- []byte(hex.Dump(b[:n]))
			c.from.Write(b[:n])
			//
			offset += n
			packet_n += 1
		}
	}
	c.from.Close()
	hc.Close()
	nd_s := Nd_session{}
	nd_s.Event = 1
	nd_s.TimeUnix = uint32(time.Now().Unix())
	nd_s.Addr = c.from.LocalAddr().String()
	nd_s.Size = offset
	nd_s.Pcount = packet_n
	avNd, err := json.Marshal(nd_s)
	if err == nil {
		c.logger <- avNd
		set_data(fmt.Sprintf("%d,%d,%d,'%s'", 2, c.time_session,
			c.id_session, avNd))

	}

	c.ack <- true // Посылаем сообщение в главный поток, что мы закончили.
}

// Данная функция обслуживает соединение. Запускает необходимые потоки и ждет
// их завершения.
func process_connection(local net.Conn, conn_n int64) {

	lt := ListTo{}
	lt.to = make([]netCon, len(tconfig.Server))
	lt.from = local
	lt.Index_head = tconfig.Index_head

	var target string
	var err error

	for i, ser := range tconfig.Server {
		// Соединяемся к удаленном сокету, куда будем пересылать данные.
		target := net.JoinHostPort(ser.Name, strconv.Itoa(ser.Port))
		lt.to[i].con, err = net.Dial("tcp", target)
		lt.to[i].active = true
		if err != nil {
			//fmt.Printf("Unable to connect to %s, %v\n", target, err)
			lt.to[i].active = false
			if i == lt.Index_head {
				local.Close()
				return
			}
		}
	}
	// Канал для получения подтверждений от качающих потоков.
	ack := make(chan bool)
	// Создаем каналы для обмена с логгерами.
	logger := make(chan []byte)
	lt.ack = ack
	lt.logger = logger

	local_info := printable_addr(lt.to[lt.Index_head].con.LocalAddr())
	remote_info := printable_addr(lt.to[lt.Index_head].con.RemoteAddr())

	// Засекаем начальное время.
	started := time.Now()

	// Запускаем логгер.
	go connection_logger(logger, conn_n, local_info, remote_info)

	logger <- []byte(fmt.Sprintf("Connected to %s at %s\n", target,
		format_time(started)))

	// Запускаем качающие потоки.
	lt.id_session = conn_n
	lt.time_session = time.Now().Unix()
	go from_to_copy(&lt) //to_logger
	go pass_through(&lt) //from_logger,

	// Ждем подтверждения об их завершении.
	<-ack
	<-ack

	// Вычисляем длительность соединения.
	finished := time.Now()
	duration := finished.Sub(started)
	logger <- []byte(fmt.Sprintf("Finished at %s, duration %s\n",
		format_time(started), duration.String()))

	// Посылаем логгерам команды закругляться. Мы тут не ждем от них
	// подтверждения, так как они и так завершатся рано или поздно, а они нам
	// более не нужны.
	logger <- []byte{}

}
func run_proxy() {
	list := fmt.Sprintf(":%d", tconfig.LocalPort)
	// слушаем порт
	ln, err := net.Listen("tcp", list)
	if err != nil {
		fmt.Printf("Unable to start listener, %v\n", err)
		os.Exit(1)
	}
	var conn_n int64 = 1
	fmt.Printf("From  %s -> to -> \n", list)
	for _, t := range tconfig.Server {
		fmt.Printf("%s : %d \n", t.Name, t.Port)
	}
	for {
		// Ждем новых соединений.
		if conn, err := ln.Accept(); err == nil {
			// Запускаем поток обработки соединения.
			go process_connection(conn, conn_n)
			conn_n += 1
		} else {
			fmt.Printf("Accept failed, %v\n", err)
		}
	}
}
func main() {

	map_con = make(map[int64]con_work, 10000)
	map_bnst = make(map[int32]int64, 10000)

	// Просим Go использовать все имеющиеся в системе процессоры.
	runtime.GOMAXPROCS(runtime.NumCPU())
	//Открываем конфиг
	var err error
	tconfig, err = tconfig.Open_cfg(fname)
	// go storage.SaveDB(tconfig, writeData) //saveDB()

	if err != nil {
		fmt.Println("Error open config: ", err)
		os.Exit(1)
	}

	// запускаем прокси
	go run_proxy()

	router := mux.NewRouter()

	router.HandleFunc("/", IndexHandler)
	router.HandleFunc("/edit/{id:[0-9]+}", SetParamCom).Methods("GET")
	router.HandleFunc("/edit/{id:[0-9]+}", SendCommand).Methods("POST")
	//func SendCommand(w http.ResponseWriter, r *http.Request) {

	router.HandleFunc("/del/{id:[0-9]+}", DiscHandler)

	http.Handle("/templates/css/", http.StripPrefix("/templates/css", http.FileServer(http.Dir("./templates/css/"))))
	http.Handle("/templates/js/", http.StripPrefix("/templates/js", http.FileServer(http.Dir("./templates/js/"))))

	http.Handle("/", router)
	portControl := fmt.Sprintf(":%d", tconfig.PortControl)
	fmt.Printf("Server is listening %s\n", portControl)

	http.ListenAndServe(portControl, nil)
}
