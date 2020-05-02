# torblocker
Service collects Tor exit nodes and prepare conf format to block in NGINX

### How to add to NGINX
To your `nginx.conf` add line
```shell script
http {
  ...
  # blocked IPs
  include /etc/nginx/block/*.conf;
}
```

Create directory `block`
```shell script
mkdir -p /etc/nginx/block/
```

Add crontab rule
```shell script
*/5 * * * * curl --output /etc/nginx/block/tor.conf http://localhost:8091/
```