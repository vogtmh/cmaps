DATE=$(date +%Y-%m-%d-%H%M%S)

mysqldump -uroot -phU2U3Jgo47kh --all-databases --events --ignore-table=mysql.event > /root/backupdb/all_dbs-$DATE.sql
find /root/backupdb/ -mtime +30 -exec rm {} \;
