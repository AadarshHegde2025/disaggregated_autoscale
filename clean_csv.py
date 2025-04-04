import pandas as pd 
import sqlite3   

def create_db(): # run to create the database locally on machine
    task_df = pd.read_csv('./data/batch_task.csv', header=None)
    instance_df = pd.read_csv('./data/batch_instance.csv', header=None)

    task_columns = [
        'create_timestamp', 'modify_timestamp', 'job_id', 'task_id',
        'instance_num', 'status', 'plan_cpu', 'plan_mem'
    ]
    instance_columns = [
        'start_timestamp', 'end_timestamp', 'job_id', 'task_id',
        'machineID', 'status', 'seq_no', 'total_seq_no',
        'real_cpu_max', 'real_cpu_avg', 'real_mem_max', 'real_mem_avg'
    ]

    task_df.columns = task_columns
    instance_df.columns = instance_columns

    conn = sqlite3.connect('batch_data.db')

    task_df.to_sql('tasks', conn, index=False, if_exists='replace')
    instance_df.to_sql('instances', conn, index=False, if_exists='replace')

def merge():
    task_df = pd.read_csv('./data/batch_task.csv', header=None)
    instance_df = pd.read_csv('./data/batch_instance.csv', header=None)

    task_columns = [
        'create_timestamp', 'modify_timestamp', 'job_id', 'task_id',
        'instance_num', 'status', 'plan_cpu', 'plan_mem'
    ]
    instance_columns = [
        'start_timestamp', 'end_timestamp', 'job_id', 'task_id',
        'machineID', 'status', 'seq_no', 'total_seq_no',
        'real_cpu_max', 'real_cpu_avg', 'real_mem_max', 'real_mem_avg'
    ]

    merged_df = pd.merge(
        task_df,
        instance_df,
        on=['job_id', 'task_id'],
        how='inner',
        suffixes=('_task', '_instance')
    )

    # Save the merged data
    merged_df.to_csv('merged_batch_data.csv', index=False)

    print(f"Merged data shape: {merged_df.shape}")
    print(f"Columns in merged data: {merged_df.columns.tolist()}")


def clean_db():
    conn = sqlite3.connect('batch_data.db')
    cursor = conn.cursor()

    # Begin transaction
    cursor.execute("BEGIN TRANSACTION;")

    cursor.execute("""
        DELETE FROM instances
        WHERE job_id IS NULL OR task_id IS NULL;
    """)

    cursor.execute("""
        DELETE FROM instances
        WHERE status != 'Terminated';
    """)

    cursor.execute("""
        DELETE FROM instances
        WHERE real_cpu_max IS NULL OR real_mem_max IS NULL;
    """)

    cursor.execute("""
        DELETE FROM instances
        WHERE rowid NOT IN (
            SELECT MIN(rowid)
            FROM instances
            GROUP BY job_id, task_id
        );
    """)

    # Delete from tasks tabla

    cursor.execute("""
        DELETE FROM tasks
        WHERE job_id IS NULL OR task_id IS NULL;
    """)

    cursor.execute("""
        DELETE FROM tasks
        WHERE status != 'Terminated';
    """)

    cursor.execute("""
        DELETE FROM tasks
        WHERE rowid NOT IN (
            SELECT MIN(rowid)
            FROM tasks
            GROUP BY job_id, task_id
        );
    """)

    cursor.execute("""
        DELETE FROM instances
        WHERE (job_id, task_id) NOT IN (
            SELECT job_id, task_id FROM tasks
        );
    """)

    cursor.execute("""
        DELETE FROM tasks
        WHERE (job_id, task_id) NOT IN (
            SELECT job_id, task_id FROM instances
        );
    """)

    

    # Commit changes
    conn.commit()
    cursor.execute("SELECT COUNT(*) FROM instances;")
    total_rows = cursor.fetchone()[0]

    print(f"Database cleaned. Remaining rows in 'instances': {total_rows}")

    cursor.execute("SELECT COUNT(*) FROM tasks;")
    total_rows = cursor.fetchone()[0]

    print(f"Database cleaned. Remaining rows in 'tasks': {total_rows}")
    conn.close()

    print("Database cleaned successfully.")


if __name__ == "__main__":

    # Database creation
    # create_db()
    clean_db()

    # Data merging 
    # merge()

    # Data cleaning

    # merged_df = pd.read_csv('./data/cleaned_file.csv')
    # cleaned_df = merged_df.dropna(subset=['plan_cpu', 'plan_mem'])

    # # Save to new CSV
    # cleaned_df.to_csv('cleaned_file.csv', index=False)
    # print(merged_df['plan_cpu'])
    # deduped_df = merged_df.drop_duplicates(subset=['job_id', 'task_id'], keep='first')
    # deduped_df.to_csv("cleaned_merged_output.csv", index=False)
    # # instance_df = instance_df.drop(columns=["real_cpu_max", "real_cpu_avg", "real_mem_max", "real_mem_avg"])
    # # instance_df = instance_df.drop_duplicates(subset=["job_id"], keep="first")
    # instance_df = instance_df[~instance_df["status_task"].isin(["Terminated", "Failed", "Cancelled"])]
    # instance_df = instance_df[~instance_df["status_instance"].isin(["Terminated", "Failed", "Cancelled"])]

    # instance_df.to_csv("cleaned_merged_output.csv", index=False)