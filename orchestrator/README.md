1) Create a .env file in this directory as follows
```
SSH_HOST_BEGIN = [e.g [TERM]-[CLASS]-0901.cs.illinois.edu]   
SSH_PORT= 22  
SSH_USER= [YOUR NET ID]  
SSH_PASS= [YOUR PASSWORD]  
```

2) Pip install the necessary python modules

```
pip install paramiko
pip install python-dotenv
pip install pyyaml
```

OR 
```
pip install -r requirements.txt
```

3) Modify the config.yaml accordingly 

4) Run orchestrator.py 
```
orchestrator.py -o [OUTPUT FILE] -c [CONFIG FILE]
```