Repo create
```bash
curl -v -X POST http://localhost:8088/api/repo \
   -H "Content-Type: application/json" \
   -d '{"name": "features"}'
```

Get repos list
```bash
curl http://localhost:8088/api/repos
```

Init repo
``` 
git clone http://localhost:8088/features.git ./flipt-demo-repo
cd flipt-demo-repo/
echo 'enabled: true' > flags.yaml
git add . && git commit -m "init"
git push origin main

aws --endpoint-url=http://localhost:4566 \
   --region us-east-1 \
   s3 ls s3://git-repos/ --recursive
```

Check log
``` 
> git clone http://localhost:8088/onoffinc-features.git 

Cloning into 'onoffinc-features'...
Unpacking objects: 100% (15/15), 2.50 KiB | 851.00 KiB/s, done.

> cd onoffinc-features/

> git log

commit 5c84f46ef454610a6e5a377a967da2e379d8943c (HEAD -> main, origin/main, origin/HEAD)
Author: flipt <flipt[bot]@noreply.flipt.io>
Date:   Fri Jun 19 13:20:17 2026 +0000

    update flag default/real-time-availability

commit ddc48ab611605e5def6c0760710f9734a6a1e9cb
Author: flipt <flipt[bot]@noreply.flipt.io>
Date:   Fri Jun 19 13:20:10 2026 +0000

    update flag default/sale

commit fd6562f7d24f9a1afcd37b1ce9b59e9fd4dd944b
Author: init <init@local>
Date:   Fri Jun 19 13:19:18 2026 +0000

    initial commit

commit e69c5d575bf1194b6215ca26d111641a0b0cbaf4
Author: Git Server <git-server@example.com>
Date:   Fri Jun 19 13:19:18 2026 +0000

    Initial commit
    
    Created repository with README.md
```
