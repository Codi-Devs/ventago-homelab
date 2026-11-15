# Copias de MySQL en el home lab

La laptop jala cada minuto el spool cifrado del VPS y, el domingo a las
04:00 hora de Panamá (09:00 UTC), restaura la última foto en un MySQL
temporal para comprobar que se puede abrir. No abre puertos y no borra las
copias al probar.

`scripts/deploy.sh` copia esta carpeta junto con el orquestador, pero no
activa los timers ni reinicia nada de este respaldo. Activarlos es un paso
aparte.

## En la laptop

```sh
sudo apt-get install -y rsync age zstd
mkdir -p /home/ventago/backups/mysql
install -m 600 /dev/null /home/ventago/homelab/mysql-backup.env
# completar mysql-backup/.env.example
# la clave privada age, chmod 600, en AGE_IDENTITY
```

Tres checks en Healthchecks. Los correos se configuran en esa cuenta, no en este repo:

| Check | Periodo | Gracia |
|---|---|---|
| jalón | 1 min | 10 min |
| foto | 1 día | 2 h |
| restore | 7 días | 1 día |

Una conexión que no responde se corta a los 30 segundos. El jalón del
binlog no pasa de 60 segundos.

La foto va por `pull-full.sh`, una vez por hora, sin límite de tiempo.
Si dejan de llegar bytes, esa descarga se corta y sigue desde el último
byte. Una base más grande tarda más minutos y termina igual.

La cuenta `ventago` tiene que poder usar Docker para la prueba semanal.
La imagen `mysql:8.0.43` no trae `mysqlbinlog`. La prueba construye
`mysql-backup-binlog:8.0.43` la primera vez, a partir de `Dockerfile.binlog`.

```sh
sudo cp mysql-backup/mysql-backup-pull.service mysql-backup/mysql-backup-pull.timer \
  mysql-backup/mysql-backup-full.service mysql-backup/mysql-backup-full.timer \
  mysql-backup/mysql-backup-restore.service mysql-backup/mysql-backup-restore.timer \
  /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now mysql-backup-pull.timer mysql-backup-full.timer mysql-backup-restore.timer
```
