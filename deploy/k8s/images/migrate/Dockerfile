FROM mysql:8.4

COPY deploy/compose/init/mysql/00-databases.sql /opt/init/00-databases.sql
COPY deploy/compose/init/mysql/02-dtm.sql /opt/init/02-dtm.sql
COPY migrations /migrations
COPY deploy/k8s/images/migrate/run.sh /run.sh
RUN chmod +x /run.sh

ENTRYPOINT ["/run.sh"]
