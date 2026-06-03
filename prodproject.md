## what to do
1. using this current front backend application, but use the aurora mysql as database instead.
frontbackend is deployed on the remote ssh machine. 
it connects to aurora mysql db.

2. use ai and import 100 japanese blogs.

3. use sysbench or playwright or any tools, to simulate a write-read workload of this website: write blog, read blog.
we just run the workload on the ssh remote machine.
we can control the duration of this workload. let us just set it to 4 hours.
4. while doing 3, we can

## resources:
[1] source aurora mysql
the connection info is:
the password is password.

```
curl -o global-bundle.pem https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem
mysql -h yanzhouw-newhire-test-source.cluster-cdximlzkzbgd.ap-northeast-1.rds.amazonaws.com -P 0 -u admin -p --ssl-mode=VERIFY_IDENTITY --ssl-ca=./global-bundle.pem
````


[2] client ssh remote machine
the ssh key is in Downloads/
ssh -i "yanzhou.pem" ec2-user@ec2-13-231-221-52.ap-northeast-1.compute.amazonaws.com

[3] TiDB connection information
mysql --comments -u 'root' -h privatelink-20616390.mczxoo2az8r7.clusters.tidb-cloud.com -P 4000 -D 'test' -p'password'

