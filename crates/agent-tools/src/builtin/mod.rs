pub mod ask;
pub mod checks;
pub mod dependency;
pub mod exec;
pub mod fs;
pub mod git;
pub mod inspect;
pub mod lsp;
pub mod net;
pub mod paths;
pub mod plan;
pub mod process;
pub mod terminal;

use crate::registry::ToolRegistry;

pub fn register_builtin(registry: ToolRegistry) -> ToolRegistry {
    registry
        .register(fs::read_file_tool())
        .register(fs::write_file_tool())
        .register(fs::edit_file_tool())
        .register(fs::patch_file_tool())
        .register(fs::apply_patch_tool())
        .register(fs::delete_path_tool())
        .register(fs::list_directory_tool())
        .register(fs::glob_tool())
        .register(fs::search_tool())
        .register(inspect::read_many_files_tool())
        .register(inspect::view_image_tool())
        .register(net::http_fetch_tool())
        .register(checks::run_checks_tool())
        .register(plan::update_plan_tool())
        .register(ask::ask_user_tool())
        .register(terminal::shell_tool())
        .register(process::process_tool())
        .register(dependency::dependency_tool())
        .register(lsp::lsp_tool())
        .register(lsp::lsp_rename_tool())
        .register(git::git_tool())
}
