use std::io::Result;

fn main() -> Result<()> {
    prost_build::Config::new()
        .bytes(["."])
        .compile_protos(&["../proto/internal.proto"], &["../proto"])?;
    Ok(())
}
