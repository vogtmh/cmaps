# Dockerfile for CompanyMaps 
# http://www.mavodev.de

# Example:
# docker run -it --rm -p 80:80 cmaps:latest

# Build with:
# docker build . -f docker/Dockerfile -t cmaps:latest

FROM mattrayner/lamp

MAINTAINER Maximilian Vogt <info@mavodev.de>

ADD source /var/www/html/
#RUN apt update && apt install -y vim
RUN chmod -R 777 /var/www/html && rm /var/www/html/config.php && mv /var/www/html/democonfig.php /var/www/html/config.php && tail /var/www/html/config.php

#RUN apt-get install -y apache2 mysql-server php7.2 libapache2-mod-php7.2 php-mysql php-curl php-json php-cgi phpmyadmin php-mbstring php-gettext

# Rethinkdb process
#EXPOSE 28015
# Rethinkdb admin console
#EXPOSE 8080

# Create the /rethinkdb_data dir structure
#RUN /usr/bin/rethinkdb create

#ENTRYPOINT ["/usr/bin/rethinkdb"]

#CMD ["--help"]