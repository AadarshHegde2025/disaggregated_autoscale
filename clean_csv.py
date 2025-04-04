import pandas as pd    

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

    task_df.columns = task_columns
    instance_df.columns = instance_columns

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

if __name__ == "__main__":

    # Data merging 
    # merge()

    # Data cleaning

    merged_df = pd.read_csv('./data/cleaned_file.csv')
    # cleaned_df = merged_df.dropna(subset=['plan_cpu', 'plan_mem'])

    # # Save to new CSV
    # cleaned_df.to_csv('cleaned_file.csv', index=False)
    print(merged_df['plan_cpu'])
    # deduped_df = merged_df.drop_duplicates(subset=['job_id', 'task_id'], keep='first')
    # deduped_df.to_csv("cleaned_merged_output.csv", index=False)
    # # instance_df = instance_df.drop(columns=["real_cpu_max", "real_cpu_avg", "real_mem_max", "real_mem_avg"])
    # # instance_df = instance_df.drop_duplicates(subset=["job_id"], keep="first")
    # instance_df = instance_df[~instance_df["status_task"].isin(["Terminated", "Failed", "Cancelled"])]
    # instance_df = instance_df[~instance_df["status_instance"].isin(["Terminated", "Failed", "Cancelled"])]

    # instance_df.to_csv("cleaned_merged_output.csv", index=False)