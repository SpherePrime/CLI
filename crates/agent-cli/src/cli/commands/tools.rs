use std::process::ExitCode;

pub fn run() -> ExitCode {
    println!("Available tools:");
    println!("  read_file       read a file");
    println!("  write_file      write a file");
    println!("  edit_file       edit a file with search/replace or line-range");
    println!("  delete_file     delete a file");
    println!("  list_directory  list a directory");
    println!("  search_files    search files by glob");
    println!("  find_files      find files by glob");
    println!("  shell           run a shell command");
    println!("  run_command     run a command");
    ExitCode::SUCCESS
}
