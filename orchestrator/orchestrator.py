import paramiko
import time
import select
from dotenv import load_dotenv
import os
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


# Handles multiple ssh sessions concurrently through the use of multiprocessing
def runOrchestrator(configFile, outputFile="output.txt"):
    processes = []
    queues = []

    # Erase contents of outputFile
    with open(outputFile, 'w') as file:
        pass

    print("Spawning shell processes...")

    ack_queue = Queue()
    file_lock = Lock()

    vm_hosts = getOnlineVMs(configFile)
    vm_commands = readCommandsFromConfig(configFile)

    sorted_vm_numbers = sorted(vm_hosts.keys())  # Optional: For consistent ordering

    for vm_number in sorted_vm_numbers:
        hostname = vm_hosts[vm_number]
        input_queue = Queue()

        process = Process(
            target=ssh_streaming_session,
            args=(
                hostname,
                PORT,
                USERNAME,
                PASSWORD,
                outputFile,
                vm_number,
                input_queue,
                ack_queue,
                file_lock
            )
        )
        queues.append((vm_number, input_queue))
        processes.append(process)
        process.start()
        time.sleep(0.2)

    print("Shells created, running commands...")

    while True:
        # Re-parse the config
        vm_commands = readCommandsFromConfig(configFile)

        # Send commands
        for vm_number, command_list in vm_commands.items():
            for q_vm, queue in queues:
                if q_vm == vm_number:
                    for command in command_list:
                        queue.put(command)
                    queue.put("STOP")
                    break

        # Wait for all acknowledgments
        done_count = 0
        while done_count < len(vm_commands):
            worker_id, msg = ack_queue.get()
            if msg == "DONE":
                print(f"Main: Worker-{worker_id} has finished.")
                done_count += 1

        prompt = "q to Quit or r to Reparse and run commands in Config File or rc to rerun and clear output file: "
        response = input(f"Commands Executed, see {outputFile} for details. {prompt}")
        while response not in ('q', 'r', 'rc'):
            response = input(f"Input not recognized. {prompt}")

        if response == 'q':
            for _, queue in queues:
                queue.put("TERMINATE")
            break
        elif response == 'r':
            continue
        elif response == 'rc':
            with open(outputFile, 'w') as file:
                pass
            continue

    for process in processes:
        process.join()
    print(f"Shells terminated, see {outputFile} for latest output")


def readCommandsFromConfig(configFile):
    with open(configFile, 'r') as file:
        configData = yaml.safe_load(file)

    vm_commands = {}
    for vmType in configData:
        for vmNumber in vmType['vm_numbers']:
            vm_commands[vmNumber] = vmType['commands']
    return vm_commands


def getOnlineVMs(configFile):
    global HOST_BEGIN
    with open(configFile, 'r') as file:
        configData = yaml.safe_load(file)

    vm_hosts = {}
    for vmType in configData:
        for vmNumber in vmType['vm_numbers']:
            hostname = f"{HOST_BEGIN[:13]}{('' if len(str(vmNumber)) == 2 else '0')}{vmNumber}{HOST_BEGIN[15:]}"
            vm_hosts[vmNumber] = hostname
    return vm_hosts


def ssh_streaming_session(host, port, user, password, outputFile, VMNumber, input_queue, ack_queue, file_lock):
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())

    try:
        client.connect(host, port, user, password)
        shell = client.invoke_shell()
        time.sleep(1)

        while True:
            command = input_queue.get()
            if command == "STOP":
                ack_queue.put((VMNumber, "DONE"))
            elif command == "TERMINATE":
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
            outputList = output.split("\n")
            for data in outputList:
                if "Last login" in data or user in data or command in data:
                    continue
                writeToOutputFile(outputFile, VMNumber, data, file_lock)

            if output.endswith("$ ") or output.endswith("# ") or output.endswith("> "):
                break


def writeToOutputFile(filename, VMNumber, data, file_lock):
    with file_lock:
        with open(filename, 'a') as file:
            file.write(f"VM{VMNumber}: {data} \n")


if __name__ == "__main__":
    freeze_support()
    parser = argparse.ArgumentParser()
    parser.add_argument('-o', '--output', type=str, help="Output File name")
    parser.add_argument('-c', '--config', type=str, help="Config File name")
    args = parser.parse_args()

    if args.config and args.output:
        runOrchestrator(configFile=args.config, outputFile=args.output)
    elif args.config:
        runOrchestrator(configFile=args.config)
    else:
        print("Config file not supplied. Correct invocation: orchestrator.py -c [CONFIG FILE] (-o [OUTPUT FILE])")