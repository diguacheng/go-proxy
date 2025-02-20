package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

var (
	host       string
	localPort  int
	remotePort int
)

func init() {
	flag.StringVar(&host, "h", "39.102.81.17", "remote server ip")
	//flag.StringVar(&host, "h", "127.0.0.1", "remote server ip")
	flag.IntVar(&localPort, "l", 7860, "the local port")
	flag.IntVar(&remotePort, "r", 5344, "remote server port")
}

type server struct {
	conn net.Conn
	// 数据传输通道
	read  chan []byte
	write chan []byte
	// 异常退出通道
	exit chan error
	// 重连通道
	reConn chan bool
}

// 从Server端读取数据
func (s *server) Read(ctx context.Context) {

	for {
		// 如果10秒钟内没有消息传输，则Read函数会返回一个timeout的错误
		_ = s.conn.SetReadDeadline(time.Now().Add(time.Second * 10))
		select {
		case <-ctx.Done():
			return
		default:
			data := make([]byte, 10240)
			n, err := s.conn.Read(data)
			if err != nil {
				if err != io.EOF {
					// 读取超时，发送一个心跳包过去
					if strings.Contains(err.Error(), "timeout") {
						// 3秒发一次心跳
						_ = s.conn.SetReadDeadline(time.Now().Add(time.Second * 3))
						s.conn.Write([]byte("pi"))
						continue
					}
					log.Println("从server读取数据失败, ", err.Error())
					s.exit <- err
					return
				} else {
					return
				}
			}

			// 如果收到心跳包, 则跳过
			if data[0] == 'p' && data[1] == 'i' {
				log.Println("client收到心跳包")
				continue
			}
			log.Printf("server reade %s\n", string(data[:n]))
			s.read <- data[:n]
		}
		log.Println("server Read....")
	}
}

// 将数据写入到Server端
func (s *server) Write(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 10)
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-s.write:
			_, err := s.conn.Write(data)
			if err != nil && err != io.EOF {
				log.Println("将数据写入server失败 ", err.Error())
				s.exit <- err
				return
			}
		case <-ticker.C:
			_, err := s.conn.Write([]byte("pi"))
			if err != nil {
				log.Println(err)
			}

		}

		log.Println("server Write....")
	}
}

type local struct {
	conn net.Conn
	// 数据传输通道
	read  chan []byte
	write chan []byte
	// 有异常退出通道
	exit chan error
}

func (l *local) Read(ctx context.Context) {

	for {
		_ = l.conn.SetReadDeadline(time.Now().Add(time.Second * 10))
		select {
		case <-ctx.Done():
			return
		default:
			data := make([]byte, 10240)
			n, err := l.conn.Read(data)
			log.Println("read", n, err)
			if err != nil {
				if err != io.EOF {
					//if strings.Contains(err.Error(), "timeout") {
					//	// 3秒发一次心跳
					//	_ = l.conn.SetReadDeadline(time.Now().Add(time.Second * 3))
					//	l.conn.Write([]byte("pi"))
					//	continue
					//}
					log.Println("local读取数据失败", err.Error())
					l.exit <- err
					return
				} else {
					return
				}
			}
			log.Printf("local reade n:%d %s\n", n, string(data[:n]))
			l.read <- data[:n]
		}
		log.Println("local Read....")
	}
}

func (l *local) Write(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-l.write:
			_, err := l.conn.Write(data)
			if err != nil {
				log.Println("local写入数据失败", err.Error(), string(data))
				l.exit <- err
				return
			}
		}
		log.Println("local Write....")
	}
}

func main() {
	flag.Parse()

	target := net.JoinHostPort(host, fmt.Sprintf("%d", remotePort))
	for {
		serverConn, err := net.Dial("tcp", target)
		if err != nil {
			panic(err)
		}
		log.Printf("已连接server: %s\n", serverConn.RemoteAddr())

		server := &server{
			conn:   serverConn,
			read:   make(chan []byte),
			write:  make(chan []byte),
			exit:   make(chan error),
			reConn: make(chan bool),
		}

		go handle(server)
		<-server.reConn
		//_ = server.conn.Close()
	}

}

func handle(server *server) {
	// 等待server端发来的信息，也就是说user来请求server了
	ctx, cancel := context.WithCancel(context.Background())

	go server.Read(ctx)
	go server.Write(ctx)

	localConn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		panic(err)
	}

	local := &local{
		conn:  localConn,
		read:  make(chan []byte),
		write: make(chan []byte),
		exit:  make(chan error),
	}

	go local.Read(ctx)
	go local.Write(ctx)

	defer func() {
		_ = server.conn.Close()
		_ = local.conn.Close()
		server.reConn <- true
	}()

	for {
		select {
		case data := <-server.read:
			local.write <- data

		case data := <-local.read:
			server.write <- data

		case err := <-server.exit:
			log.Printf("server have err: %s\n", err.Error())
			cancel()
			return
		case err := <-local.exit:
			log.Printf("local have err: %s\n", err.Error())
			cancel()
			return
		}
	}
}
