import type { Project } from '../../../../shared/contracts'

export function projectName(project: Project | null | undefined): string | undefined {
  if (!project) return undefined
  if (project.remote)
    return `${project.remote.name} · ${project.remote.root.split('/').filter(Boolean).pop() || '/'}`
  return project.path.split(/[\\/]/).pop()
}

export function projectRoot(project: Project): string {
  return project.remote?.root ?? project.path
}