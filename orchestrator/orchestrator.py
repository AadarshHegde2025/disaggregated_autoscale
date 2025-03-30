import paramiko
import time
import select
from dotenv import load_dotenv
import os
import time
from multiprocessing import Process, freeze_support
import threading
import yaml
import argparse


# Load credentials from .env
load_dotenv()

HOST_BEGIN = os.getenv("SSH_HOST_BEGIN")
PORT = int(os.getenv("SSH_PORT", 22))
USERNAME = os.getenv("SSH_USER")
PASSWORD = os.getenv("SSH_PASS")

# Dependent on VM configuration, this works to 
# accumulate list of VM hostnames to connect to
HOST_LIST = []
for i in range(20):
    HOST_LIST.append(f"{HOST_BEGIN[:13]}{("" if len(f"{i + 1}") == 2 else "0")}{i + 1}{HOST_BEGIN[15:]}")

# Create a global mutex lock for the output file
file_lock = threading.Lock()

# Handles multiple ssh sessions concurrently through the use of multiprocessing
def runOrchestrator(configFile, outputFile = "output.txt"):

    processes = []
    # Erase contents of outputFile
    with open(outputFile, 'w') as file: pass  

    print("Spawning shell processes...")
    
    # Read command data from config file 
    configData = None
    commands = [[]]* 20
    with open(configFile, 'r') as file:
        configData = yaml.safe_load(file)

    for vmType in configData:
        for vmNumber in vmType['vm_numbers']:
            commands[vmNumber - 1] = vmType['commands']
    
    for i in range(len(HOST_LIST)):
        # Spawn a new process with a distinct VM number for logging
        process = Process(
            target=ssh_streaming_session,
            args=(
                HOST_LIST[i],
                PORT,
                USERNAME,
                PASSWORD,
                outputFile,
                i + 1,
                commands[i]
            )
        )
        processes.append(process)
        process.start()
    
    print("Shells created, running commands...")

    # Wait for all processes to complete
    for process in processes:
        process.join()
    print(f"Commands executed, see {outputFile} for details")

# Handles piping to single ssh session
def ssh_streaming_session(host, port, user, password, outputFile, VMNumber, commands):
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())

    try:
        # Connect to the SSH server
        client.connect(host, port, user, password)

        # Start an interactive shell session
        shell = client.invoke_shell()
        time.sleep(1)  # Allow time for shell to warmup

        for command in commands:
            sendCommandToShell(shell, command, user, VMNumber, outputFile)

    finally:
        client.close()

def sendCommandToShell(shell, command, user, VMNumber, outputFile):
    shell.send(command + "\n")            
    time.sleep(0.2)
    while True:
        r, _, _ = select.select([shell], [], [], 0.5)
        
        if shell in r:
            output = shell.recv(1024).decode()

            # Filter lines with extraneous ouput
            outputList = output.split("\n")
            for data in outputList:
                if "Last login" in data or f"{user}" in data or command in data:
                    continue
                writeToOutputFile(outputFile, VMNumber, data)

            # Break out if we detect the prompt (basic heuristic)
            if output.endswith("$ ") or output.endswith("# ") or output.endswith("> "):
                break

def writeToOutputFile(filename, VMNumber, data):
    with file_lock:
        with open(filename, 'a') as file:
            # Data cleaned to omit bracketed paste mode (See more here [https://en.wikipedia.org/wiki/Bracketed-paste])
            file.write(f"VM{VMNumber}: {data.split("\r")[1]} \n")

if __name__ == "__main__":
    # Required for compatibility on windows
    freeze_support()

    parser = argparse.ArgumentParser()
    parser.add_argument('-o', '--output', type=str, help="Output File name")
    parser.add_argument('-c', '--config', type=str, help="Config File name")

    args = parser.parse_args()
    if(args.config and args.output):
        runOrchestrator(configFile= args.config, outputFile= args.ouput)
    elif(args.config):
        runOrchestrator(configFile= args.config)
    else:
        print("Config file not supplied. Correct invocation: orchestrator.py -c [CONFIG FILE] (-o [OUTPUT FILE])")