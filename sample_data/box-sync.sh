#!/bin/bash

mount /mnt/box
rsync -avz --recursive --delete /mnt/box/Service/EmployeePictures/ /var/www/html/avatarcache/
umount /mnt/box
# Convert uppercase to lowercase (JPG -> jpg)
rename 'y/A-Z/a-z/' *
