pub mod fs;
pub mod git;
pub mod inspect;
pub mod paths;
pub mod terminal;

use crate::registry::ToolRegistry;

pub fn register_builtin(registry: ToolRegistry) -> ToolRegistry {
    registry
        .register(fs::read_file_tool())
        .register(fs::write_file_tool())
        .register(fs::edit_file_tool())
        .register(fs::patch_file_tool())
        .register(fs::delete_path_tool())
        .register(fs::list_directory_tool())
        .register(fs::glob_tool())
        .register(fs::search_tool())
        .register(inspect::read_many_files_tool())
        .register(inspect::view_image_tool())
        .register(terminal::shell_tool())
        .register(git::git_tool())
}
