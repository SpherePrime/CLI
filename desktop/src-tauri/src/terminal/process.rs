use crate::error::{HostError, HostResult};

#[cfg(windows)]
pub struct ProcessTree {
    handle: usize,
}

#[cfg(windows)]
impl ProcessTree {
    pub fn attach(pid: u32) -> HostResult<Self> {
        use windows::Win32::{
            Foundation::CloseHandle,
            System::{
                JobObjects::*,
                Threading::{OpenProcess, PROCESS_SET_QUOTA, PROCESS_TERMINATE},
            },
        };
        unsafe {
            let job = CreateJobObjectW(None, None).map_err(failure)?;
            let result = (|| {
                let mut limits = JOBOBJECT_EXTENDED_LIMIT_INFORMATION::default();
                limits.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
                SetInformationJobObject(
                    job,
                    JobObjectExtendedLimitInformation,
                    &limits as *const _ as _,
                    std::mem::size_of_val(&limits) as u32,
                )
                .map_err(failure)?;
                let process = OpenProcess(PROCESS_SET_QUOTA | PROCESS_TERMINATE, false, pid)
                    .map_err(failure)?;
                let result = AssignProcessToJobObject(job, process).map_err(failure);
                let _ = CloseHandle(process);
                result
            })();
            if let Err(error) = result {
                let _ = CloseHandle(job);
                return Err(error);
            }
            Ok(Self {
                handle: job.0 as usize,
            })
        }
    }

    pub fn terminate(&self) -> HostResult<()> {
        use windows::Win32::{
            Foundation::HANDLE,
            System::JobObjects::{
                JobObjectBasicAccountingInformation, QueryInformationJobObject, TerminateJobObject,
                JOBOBJECT_BASIC_ACCOUNTING_INFORMATION,
            },
        };
        unsafe {
            let handle = HANDLE(self.handle as _);
            TerminateJobObject(handle, 1).map_err(failure)?;
            let deadline = std::time::Instant::now() + std::time::Duration::from_secs(5);
            loop {
                let mut accounting = JOBOBJECT_BASIC_ACCOUNTING_INFORMATION::default();
                QueryInformationJobObject(
                    Some(handle),
                    JobObjectBasicAccountingInformation,
                    &mut accounting as *mut _ as _,
                    std::mem::size_of_val(&accounting) as u32,
                    None,
                )
                .map_err(failure)?;
                if accounting.ActiveProcesses == 0 {
                    return Ok(());
                }
                if std::time::Instant::now() >= deadline {
                    return Err(HostError::new(
                        "TERMINAL_TIMEOUT",
                        "Terminal process tree did not exit within five seconds",
                    ));
                }
                std::thread::sleep(std::time::Duration::from_millis(10));
            }
        }
    }
}

#[cfg(windows)]
impl Drop for ProcessTree {
    fn drop(&mut self) {
        use windows::Win32::Foundation::{CloseHandle, HANDLE};
        unsafe {
            let _ = CloseHandle(HANDLE(self.handle as _));
        }
    }
}

#[cfg(windows)]
fn failure(error: windows::core::Error) -> HostError {
    HostError::new("TERMINAL_PROCESS", error.to_string())
}

#[cfg(unix)]
pub struct ProcessTree {
    pid: i32,
}

#[cfg(unix)]
impl ProcessTree {
    pub fn attach(pid: u32) -> HostResult<Self> {
        Ok(Self { pid: pid as i32 })
    }

    pub fn terminate(&self) -> HostResult<()> {
        unsafe extern "C" {
            fn kill(pid: i32, signal: i32) -> i32;
        }
        if unsafe { kill(-self.pid, 9) } != 0 {
            let error = std::io::Error::last_os_error();
            if error.raw_os_error() != Some(3) {
                return Err(error.into());
            }
        }
        Ok(())
    }
}
