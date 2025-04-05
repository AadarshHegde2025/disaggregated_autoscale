import paramiko
import time
import select
from dotenv import load_dotenv
import os
import time
from multiprocessing import Process, freeze_support, Lock, Queue
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
# HOST_LIST = []
# for i in range(20):
#     HOST_LIST.append(f"{HOST_BEGIN[:13]}{("" if len(f"{i + 1}") == 2 else "0")}{i + 1}{HOST_BEGIN[15:]}")

# Holds the ssh clients (persistent) 
clients = []



# Handles multiple ssh sessions concurrently through the use of multiprocessing
def runOrchestrator(configFile, outputFile = "output.txt"):

    processes = []
    queues = []

    # Erase contents of outputFile
    with open(outputFile, 'w') as file: pass  

    # Spawn the shell processes
    print("Spawning shell processes...")

    # Acknowledge when commands have been executed 
    ack_queue = Queue()
    file_lock = Lock()

    HOST_LIST = getOnlineVMs(configFile= configFile)
    # print(HOST_LIST)

    for i in range(len(HOST_LIST)):
        # Queue to enter commands into the shells
        input_queue = Queue()

        VM_NUMBER = int(HOST_LIST[i].split(".")[0].split("-09")[1])
        # print(VM_NUMBER)
        # Spawn a new process with a distinct VM number for logging
        process = Process(
            target=ssh_streaming_session,
            args=(
                HOST_LIST[i],
                PORT,
                USERNAME,
                PASSWORD,
                outputFile,
                VM_NUMBER,
                input_queue, 
                ack_queue,
                file_lock
            )
        )
        queues.append(input_queue)
        processes.append(process)
        process.start()
        time.sleep(0.2)
    
    print("Shells created, running commands...")

    while(True):
        # Read commands from some config file
        commands = readCommandsFromConfig(configFile)

        # Put commands in respective workers queues
        for i in range(len(commands)):
            for command in commands[i]:
                queues[i].put(command)
            # Signals there are no new commands
            queues[i].put("STOP")

        # wait for all VMS to respond
        done_count = 0
        while done_count < len(HOST_LIST):
            worker_id, msg = ack_queue.get()
            if msg == "DONE":
                print(f"Main: Worker-{worker_id} has finished.")
                done_count += 1

        # See if config file should be kept open 
        prompt = "q to Quit or r to Reparse and run commands in Config File or rc to rerun and clear output file: "
        response = input(f"Commands Executed, see {outputFile} for details. {prompt}")
        while(response != 'q' and response != 'r' and response != 'rc'):
            response = input(f"Input not recognized. {prompt}")

        if(response == 'q'):
            for i in range(len(queues)):
                queues[i].put("TERMINATE")
            break
        elif(response == 'r'):
            continue
        elif(response == 'rc'):
            with open(outputFile, 'w') as file: pass 
            continue
    
    # Wait for all processes to complete
    for process in processes:
        process.join()
    print(f"Shells terminated, see {outputFile} for latest output")


def readCommandsFromConfig(configFile):
    # Read command data from config file 
    configData = None
    HOST_LIST = getOnlineVMs(configFile)
    commands = [[]]* len(HOST_LIST)
    with open(configFile, 'r') as file:
        configData = yaml.safe_load(file)

    for vmType in configData:
        for vmNumber in vmType['vm_numbers']:
            commands[vmNumber - 1] = vmType['commands']

    return commands

def getOnlineVMs(configFile):
    global HOST_BEGIN
    configData = None
    HOST_LIST = []
    with open(configFile, 'r') as file:
        configData = yaml.safe_load(file)
    for vmType in configData:
        for vmNumber in vmType['vm_numbers']:
            HOST_LIST.append(f"{HOST_BEGIN[:13]}{("" if len(f"{vmNumber}") == 2 else "0")}{vmNumber}{HOST_BEGIN[15:]}")

    return HOST_LIST

# Handles piping to single ssh session
def ssh_streaming_session(host, port, user, password, outputFile, VMNumber, input_queue, ack_queue, file_lock):
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())

    try:
        # Connect to the SSH server
        client.connect(host, port, user, password)

        # Start an interactive shell session
        shell = client.invoke_shell()
        time.sleep(1)  # Allow time for shell to warmup

        while True:
            command = input_queue.get()
            if(command == "STOP"):
                ack_queue.put((VMNumber, "DONE"))
            elif(command == "TERMINATE"):
                break
            else:
                sendCommandToShell(shell, command, user, VMNumber, outputFile, file_lock)

    finally:
        client.close()

def sendCommandToShell(shell, command, user, VMNumber, outputFile, file_lock):
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
                writeToOutputFile(outputFile, VMNumber, data, file_lock)

            # Break out if we detect the prompt (basic heuristic)
            if output.endswith("$ ") or output.endswith("# ") or output.endswith("> "):
                break

def writeToOutputFile(filename, VMNumber, data, file_lock):
    with file_lock:
        with open(filename, 'a') as file:
            # Data cleaned to omit bracketed paste mode (See more here [https://en.wikipedia.org/wiki/Bracketed-paste])
            file.write(f"VM{VMNumber}: {data} \n")

if __name__ == "__main__":
    # Required for compatibility on windows
    freeze_support()

    parser = argparse.ArgumentParser()
    parser.add_argument('-o', '--output', type=str, help="Output File name")
    parser.add_argument('-c', '--config', type=str, help="Config File name")

    args = parser.parse_args()
    if(args.config and args.output):
        runOrchestrator(configFile= args.config, outputFile= args.output)
    elif(args.config):
        runOrchestrator(configFile= args.config)
    else:
        print("Config file not supplied. Correct invocation: orchestrator.py -c [CONFIG FILE] (-o [OUTPUT FILE])")