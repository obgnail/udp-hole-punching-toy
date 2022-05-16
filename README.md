## udp打洞玩具

1. UDP 打洞
2. 将打好的洞转为 TCP，并且新增一个新的 UDP 打洞端口作为原先端口的替代
3. 提供本地端口代理



## usage

```
python3 -m http.server 7777

cd demo/server && go run *.go -serverPort=7788

cd demo/peer && go run *.go -id=HTTP_SERVER -passive=true -serverIP=127.0.0.1 -serverPort=7788 -proxyPort=7777 

cd demo/peer && go run *.go -choose=HTTP_SERVER -passive=false -serverIP=127.0.0.1 -serverPort=7788 -proxyPort=6666 

curl http://127.0.0.1:6666
```



## sequenceDiagram

```mermaid
sequenceDiagram
		autonumber
    participant Local
    participant peerClient
    participant Server
    participant peerServer
    participant NAS
    
    Note over Local, NAS: 准备阶段
    Server ->> Server: 监听UDP注册端口
    peerServer ->> Server: 注册ID,Addr

		par par and loop
   		peerServer ->> Server: 发送heartbeat
   	end
   	
    Note over Local, NAS: 连接阶段
   	peerClient ->> Server: 注册,请求连接到其他peer
   	Server -->> peerClient: 返回peer List
   	peerClient ->> peerClient: 选择一个peer
   	
   	par 打洞
   		peerClient -x peerServer: 请求server端口(打通C->S的NAT)
      peerClient ->> Server: 请求打洞到peerServer
      Server ->> peerServer: 命令Server连接到Client
      peerServer ->> peerClient: 连接到client(打通S->C的NAT)
   	end
   	
   	loop UDP通讯若干次(确保网络通讯顺畅)
   		peerClient ->> peerServer: heatbeat
   		peerServer ->> peerClient: answer heartbeat
   	end
   	
   	peerServer ->> peerServer: 生成新的peerServer,监听其他端口
   	peerServer ->> Server: 注册新的Server(回到步骤2)
   	peerServer ->> peerServer: UDP -> TCP.listen TCP
   	peerServer ->> NAS: 开启本地端口
   	NAS -->> peerServer: 返回localConn
   	peerClient ->> peerClient: UDP -> TCP
   	peerClient ->> peerServer: Dail Server TCP(serverConn)
   	par
   		peerServer ->> peerServer: Join ServerConn and LocalConn
   	end
   	peerClient ->> peerClient: Listen Proxy TCP
   	
   	Note over Local, NAS: 使用阶段
   	Local ->> peerClient: Dial Proxy TCP
   	par
   		peerClient ->> peerClient: Join ServerConn and ProxyConn
   	end
```

