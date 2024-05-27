
# Other notes

```
HiddenServiceExportInstanceID
https://gitlab.torproject.org/legacy/trac/-/issues/32428
https://web.archive.org/web/20200619230407/https://github.com/torproject/tor/pull/1543/files
```

```go
curl --proxy socks5h://localhost:9050 http://dkforestseeaaq2dqz2uflmlsybvnq2irzn4ygyvu53oazyorednviid.onion
```

```
/etc/systemd/system/dkf.service
service darkforest restart
journalctl -u darkforest.service
journalctl --vacuum-time=2d
journalctl --vacuum-size=500M
```

```
HiddenServiceExportCircuitID haproxy
```

```
// List listening ports
sudo netstat -tulpn | grep LISTEN

// https://stackoverflow.com/a/52577158
tcpdump -ni any port 8080 -vv -s0 -w http2.pcap

// Filter GET/POST to 8080
tcpdump -ni any -vv -s0 -A  -w http5.pcap "tcp[((tcp[12:1] & 0xf0) >> 2):4] = 0x47455420 or tcp[((tcp[12:1] & 0xf0) >> 2):4] = 0x504F5354 and tcp dst port 8080"

//https://unix.stackexchange.com/a/6300
tcpflow -p -c -i eth0 port 80 | grep -oE '(GET|POST|HEAD) .* HTTP/1.[01]|Host: .*'

tcpflow -c -i lo port 8080

netstat -t -u

http://dkforestseeaaq2dqz2uflmlsybvnq2irzn4ygyvu53oazyorednviid.onion/debug/pprof/profile?seconds=10
go tool pprof -http=:8081 ~/Downloads/profile
```


```
// Count sessions
select u.username, user_id, count(*) c1 from sessions s inner join users u on u.id = s.user_id group by user_id order by c1 desc;

// session_notifications abusers
select user_id, count(n.id) c1, u.username from sessions s
left join session_notifications n on n.session_token = s.token
inner join users u on u.id = s.user_id
group by s.user_id
order by c1 desc;
```

```
-- PRAGMA foreign_keys=ON;

-- Delete users who only ever logged in once and have no messages in the chat
delete from users
where id in (
    select id
    from users u1
    where (select count(*) from sessions s where s.user_id = u1.id) <= 1
        and (select count(*) from chat_messages m where m.user_id = u1.id) = 0
        and u1.gpg_public_key = ''
        and username != '0');

-- Select users who only have 1 session
select s.created_at, u.id, u.username,
	(select count(*)from sessions s where s.user_id = u.id)
from users u
inner join sessions s on s.id = (select max(id) from sessions s where s.user_id = u.id) and s.user_id = u.id
where (select count(*) from sessions s where s.user_id = u.id) = 1
order by s.created_at asc;

-- Delete recent users
delete from users where created_at > datetime('now', '-10 Minute', 'localtime');
delete from users where created_at > datetime('now', '-6 Hour', 'localtime');

delete from chat_messages where user_id not in (select id from users);

select username, registration_duration from users order by id desc limit 10;
```

```
./monero-wallet-rpc --wallet-file /Users/n0tr1v/Monero/wallets/dkf_stage/dkf_stage.keys --daemon-address 3.10.182.182:38081 --stagenet --rpc-bind-port 6061 --password '...' --disable-rpc-login
```

```
wget https://downloads.getmonero.org/cli/linux64
tar -xf linux64
./monero-x86_64-linux-gnu-v0.18.3.1/monero-wallet-cli --stagenet --daemon-address 3.10.182.182:38081
./monero-x86_64-linux-gnu-v0.18.3.1/monero-wallet-rpc --stagenet --daemon-address 3.10.182.182:38081 --wallet-file /home/dkf/dkf-poker-stagenet.keys --rpc-bind-port 6061 --password '...' --disable-rpc-login
./monero-x86_64-linux-gnu-v0.18.3.1/monero-wallet-cli --daemon-address 18.169.212.248:18081
./monero-x86_64-linux-gnu-v0.18.3.1/monero-wallet-rpc --wallet-file /home/dkf/dkf-poker.keys --daemon-ssl-allow-any-cert --proxy 127.0.0.1:9050 --daemon-address 18.169.212.248:18081 --rpc-bind-port 6061 --password '...' --disable-rpc-login
```