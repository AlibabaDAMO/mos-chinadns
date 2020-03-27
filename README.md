# mos-chinadns

支持DoH，IPv6，[EDNS Client Subnet](https://tools.ietf.org/html/rfc7871)，根据域名和IP的分流。内置[APNIC](https://ftp.apnic.net/apnic/stats/apnic/delegated-apnic-latest)中国大陆IP表和[dnsmasq-china-list](https://github.com/felixonmars/dnsmasq-china-list)域名表。

---

- [mos-chinadns](#mos-chinadns)
  - [命令帮助](#命令帮助)
  - [json配置文件](#json配置文件)
  - [三分钟快速上手 & 预设配置](#三分钟快速上手--预设配置)
  - [更新中国大陆IP与域名列表](#更新中国大陆ip与域名列表)
  - [分流效果](#分流效果)
  - [其他细节](#其他细节)
  - [Open Source Components / Libraries](#open-source-components--libraries)

## 命令帮助

    -c string   [路径]json配置文件路径
    -dir string [路径]变更程序的工作目录
    -dir2exe    变更程序的工作目录至可执行文件的目录

    -gen string [路径]生成一个json配置文件模板至该路径
    -v          调试模式，更多的log输出
    -q          安静模式，无log
    -no-tcp     不监听tcp，只监听udp
    -no-udp     不监听udp，只监听tcp
    -cpu        使用CPU核数 

## json配置文件

<details><summary><code>json配置文件说明与示例</code></summary><br>

    {
        // [IP:端口][必需] 监听地址。
        "bind_addr": "127.0.0.1:53", 

        // [IP:端口] `local_server`地址 建议:一个低延时但会被污染大陆服务器，用于解析大陆域名。
        "local_server": "223.5.5.5:53",    

        // [URL] DoH服务器的url，如果填入，`local_server`将使用DoH协议
        "local_server_url": "https://223.5.5.5/dns-query",

        // [path] 用于验证`local_server`的PEM格式CA证书的路径。默认使用系统证书池。
        "local_server_pem_ca": "",

        // [bool] `local_server`是否屏蔽非A或AAAA请求。
        "local_server_block_unusual_type": false,

        // [IP:端口] `remote_server`地址 建议:一个无污染的服务器。用于解析非大陆域名。   
        "remote_server": "8.8.8.8:443", 

        // [URL] DoH服务器的url，如果填入，`remote_server`将使用DoH协议。
        "remote_server_url": "https://dns.google/dns-query",  

        // [path] 用于验证`remote_server`的PEM格式CA证书的路径。默认使用系统证书池。
        "remote_server_pem_ca": "", 

        // [int] 单位毫秒 `remote_server`延时启动时间。
        // 如果在设定时间(单位毫秒)后`local_server`无响应或失败，则开始请求`remote_server`。
        // 如果`local_server`延时较低，将该值设定为120%的`local_server`的延时可显著降低请求`remote_server`的次数。
        // 该选项主要用于缓解低运算力设备的压力。
        // 0表示禁用延时，请求将同时发送。
        "remote_server_delay_start": 0, 

        // [路径] `local_server`IP白名单 建议:中国大陆IP列表，用于区别大陆与非大陆结果。
        "local_allowed_ip_list": "/path/to/your/chn/ip/list", 

        // [路径] `local_server`IP黑名单 建议:希望被屏蔽的IP列表，比如运营商的广告服务器IP。
        "local_blocked_ip_list": "/path/to/your/black/ip/list",
        
        // [路径] 强制使用`local_server`解析的域名名单 建议:中国的域名。
        "local_forced_domain_list": "/path/to/your/domain/list",

        // [路径] `local_server`域名黑名单 建议:希望强制打开国外版而非中国版的域名。
        "local_blocked_domain_list": "/path/to/your/domain/list",

        // [CIDR] EDNS Client Subnet 
        "local_ecs_subnet": "1.2.3.0/24",
        "remote_ecs_subnet": "1.2.3.0/24"
    }

</details>

## 三分钟快速上手 & 预设配置

在这里下载最新版本：[release](https://github.com/IrineSistiana/mos-chinadns/releases)

[APNIC](https://ftp.apnic.net/apnic/stats/apnic/delegated-apnic-latest)中国大陆IP表`chn.list`和[dnsmasq-china-list](https://github.com/felixonmars/dnsmasq-china-list)域名表`chn_domain.list`已包含在release的zip包中。

将预设配置复制并保存至`config.json`，确保`chn.list`，`chn_domain.list`，`config.json`和`mos-chinadns`在同一目录。

用以下命令启动

    mos-chinadns -c config.json -dir2exe

<details><summary><code>预设配置1 通用 按大陆IP与域名分流 `remote_server`使用DoH</code></summary><br>

使用中国大陆IP表`chn.list`和域名表`chn_domain.list`分流。国内域名使用`阿里云DNS`解析，国际域名使用[Google DoH](https://developers.google.com/speed/public-dns/docs/doh)解析。

    {
        "bind_addr": "127.0.0.1:53",
        "local_server": "223.5.5.5:53",
        "remote_server": "8.8.8.8:443",
        "remote_server_url": "https://dns.google/dns-query",
        "local_allowed_ip_list": "./chn.list",
        "local_forced_domain_list": "./chn_domain.list"
    }

</details>

<details><summary><code>预设配置2 通用 按大陆IP与域名分流</code></summary><br>

使用中国大陆IP表`chn.list`和域名表`chn_domain.list`分流。国内域名使用`阿里云DNS`解析，国际域名使用`OpenDNS`解析。

    {
        "bind_addr": "127.0.0.1:53",
        "local_server": "223.5.5.5:53",
        "remote_server": "208.67.222.222:443",
        "local_allowed_ip_list": "./chn.list",
        "local_forced_domain_list": "./chn_domain.list"
    }

</details>

<details><summary><code>预设配置3 DoH转发模式</code></summary><br>

使用[Google DoH](https://developers.google.com/speed/public-dns/docs/doh)作为上游服务器。无分流。建议启用ECS使解析更精确。[如何启用?](#其他细节)

    {
        "bind_addr": "127.0.0.1:53",
        "remote_server": "8.8.8.8:443",
        "remote_server_url": "https://dns.google/dns-query",
    }

</details>

`mos-chinadns`的使用场景很丰富，以上配置示例能满足绝大多数需求。如需DIY配置请参阅：[完整的json配置文件与说明](#json配置文件)

## 更新中国大陆IP与域名列表

如果你想自己更新中国大陆IP与域名列表。[release_chn_ip_domain_updater.py](https://github.com/IrineSistiana/mos-chinadns/blob/master/release_chn_ip_domain_updater.py)能自动下载数据并生成`chn.list`，`chn_domain.list`到当前目录。

## 分流效果

国内域名交由`local_server`解析，无格外延时。国外域名将会由`remote_server`解析，确保无污染。

<details><summary><code>dig www.baidu.com 演示</code></summary><br>

    ubuntu@ubuntu:~$ dig www.baidu.com @192.168.1.1 -p5455

    ; <<>> DiG 9.11.3-1ubuntu1.11-Ubuntu <<>> www.baidu.com @192.168.1.1 -p5455
    ;; global options: +cmd
    ;; Got answer:
    ;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 57335
    ;; flags: qr rd ra; QUERY: 1, ANSWER: 3, AUTHORITY: 0, ADDITIONAL: 1

    ;; OPT PSEUDOSECTION:
    ; EDNS: version: 0, flags:; udp: 4096
    ;; QUESTION SECTION:
    ;www.baidu.com.			IN	A

    ;; ANSWER SECTION:
    www.baidu.com.		561	IN	CNAME	www.a.shifen.com.
    www.a.shifen.com.	250	IN	A	36.152.44.96
    www.a.shifen.com.	250	IN	A	36.152.44.95

    ;; Query time: 4 msec
    ;; SERVER: 192.168.1.1#5455(192.168.1.1)
    ;; WHEN: Sun Mar 15 18:17:55 PDT 2020
    ;; MSG SIZE  rcvd: 149

</details>

<details><summary><code>dig www.google.com 演示</code></summary><br>

    ubuntu@ubuntu:~$ dig www.google.com @192.168.1.1 -p5455

    ; <<>> DiG 9.11.3-1ubuntu1.11-Ubuntu <<>> www.google.com @192.168.1.1 -p5455
    ;; global options: +cmd
    ;; Got answer:
    ;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 2719
    ;; flags: qr rd ra; QUERY: 1, ANSWER: 6, AUTHORITY: 0, ADDITIONAL: 1

    ;; OPT PSEUDOSECTION:
    ; EDNS: version: 0, flags:; udp: 512
    ;; QUESTION SECTION:
    ;www.google.com.			IN	A

    ;; ANSWER SECTION:
    www.google.com.		280	IN	A	74.125.68.99
    www.google.com.		280	IN	A	74.125.68.105
    www.google.com.		280	IN	A	74.125.68.104
    www.google.com.		280	IN	A	74.125.68.103
    www.google.com.		280	IN	A	74.125.68.106
    www.google.com.		280	IN	A	74.125.68.147

    ;; Query time: 72 msec
    ;; SERVER: 192.168.1.1#5455(192.168.1.1)
    ;; WHEN: Sun Mar 15 18:19:20 PDT 2020
    ;; MSG SIZE  rcvd: 223

</details>

## 其他细节

**如何使用EDNS Client Subnet (ECS)**

`*_ecs_subnet` 填入自己的IP段即可启用ECS。如不详请务必留空。

启用ECS最简单的方法:

- 百度搜索`IP`，得到自己的IP地址，如`1.2.3.4`
- 将最后一位变`0`，并加上`/24`。如`1.2.3.4`变`1.2.3.0/24`
- 将`1.2.3.0/24`填入`ecs_subnet`

**DNS-over-HTTPS (DoH)**

请求方式为[RFC 8484](https://tools.ietf.org/html/rfc8484) GET。

验证DoH服务器身份，默认使用系统证书池，或通过`*_server_pem_ca`参数提供的CA证书。

**关于文件路径**

建议使用`-dir2exe`选项将工作目录设置为程序所在目录，这样的话配置文件`-c`路径和配置文件中的路径可以是相对于程序的相对路径。

如果附加`-dir2exe`后程序启动报错那就只能启动程序前手动`cd`或者使用绝对路径。

**请求流程与local_server黑白名单**

1. 如果指定了域名白名单->匹配域名->白名单中的域名将被仅发往`local_server`解析
2. 如果指定了域名黑名单->匹配域名->黑名单中的域名将被仅发往`remote_server`解析
3. 发送至`local_server`与`remote_server`解析
4. 如果请求仅由`local_server`解析->无条件接受返回结果->END
5. 如果由`remote_server`与`local_server`同时解析->`local_server`返回的空结果会被丢弃
6. 如果指定了IP黑名单->匹配`local_server`返回的IP->丢弃黑名单中的结果
7. 如果指定了IP白名单->匹配`local_server`返回的IP->丢弃不在白名单的结果
8. 接受结果->END

`local_server`的结果会根据设置进行过滤，`remote_server`的结果一定会被接受。

**域名黑/白名单格式**

采用按域向前匹配的方式，与dnsmasq匹配方式类似。每个表达式一行。

规则示例：

* `cn`相当于`*.cn`。会匹配所有以cn结尾的域名，`example.cn`，`www.google.cn`
* `google.com`相当于`*.google.com`。会匹配`www.google.com`, `www.l.google.com`，但不会匹配`www.google.cn`。

比如：

    cn
    google.com
    google.com.hk
    www.google.com.sg

**IP黑/白名单格式**

由单个IP或CIDR构成，每个表达式一行，支持IPv6，比如：

    1.0.1.0/24
    2001:dd8:1a::/48

    2.2.2.2
    2001:ccd:1a

## Open Source Components / Libraries

部分设计参考

* [ChinaDNS](https://github.com/shadowsocks/ChinaDNS): [GPLv3](https://github.com/shadowsocks/ChinaDNS/blob/master/COPYING)

依赖

* [sirupsen/logrus](https://github.com/sirupsen/logrus): [MIT](https://github.com/sirupsen/logrus/blob/master/LICENSE)
* [miekg/dns](https://github.com/miekg/dns): [LICENSE](https://github.com/miekg/dns/blob/master/LICENSE)
* [valyala/fasthttp](https://github.com/valyala/fasthttp):[MIT](https://github.com/valyala/fasthttp/blob/master/LICENSE)
