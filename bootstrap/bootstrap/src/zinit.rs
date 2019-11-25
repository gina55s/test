use failure::Error;
use std::process::Command;

type Result<T> = std::result::Result<T, Error>;

/// monitor service via name
pub fn monitor<T>(name: T) -> Result<()>
where
    T: AsRef<str>,
{
    let output = Command::new("zinit")
        .arg("monitor")
        .arg(name.as_ref())
        .output()?;
    if output.status.success() {
        return Ok(());
    }
    bail!(
        "failed to monitor service '{}': {:?}",
        name.as_ref(),
        output
    );
}
