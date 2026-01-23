#![allow(clippy::missing_safety_doc)]

extern crate alloc;
extern crate core;

use alloc::borrow::Cow;
use alloc::rc::Rc;
use alloc::vec::Vec;
use restate_sdk_shared_core::{
    AttachInvocationTarget, CoreVM, DoProgressResponse, Error, Header, HeaderMap, NonEmptyValue,
    NotificationHandle, PayloadOptions, ResponseHead, RetryPolicy, RunExitResult, TakeOutputResult,
    Target, TerminalFailure, VMOptions, Value, Version, VM,
};
use std::cell::RefCell;
use std::convert::Infallible;
use std::io::Write;
use std::mem::MaybeUninit;
use std::time::Duration;
use tracing::level_filters::LevelFilter;
use tracing::{Level, Subscriber};
use tracing_subscriber::fmt::format::FmtSpan;
use tracing_subscriber::fmt::MakeWriter;
use tracing_subscriber::layer::SubscriberExt;
use tracing_subscriber::{Layer, Registry};

// --------- Init and logging

#[export_name = "init"]
pub unsafe extern "C" fn init(level: u32) {
    std::panic::set_hook(Box::new(|panic| {
        let panic_str = format!("Core panicked: {panic}");
        log(AbiLogLevel::Error, &panic_str)
    }));
    let _ = tracing::subscriber::set_global_default(log_subscriber(level.into()));
}

#[repr(u32)]
enum AbiLogLevel {
    Trace = 0,
    Debug = 1,
    Info = 2,
    Warn = 3,
    Error = 4,
}

impl From<u32> for AbiLogLevel {
    fn from(value: u32) -> Self {
        match value {
            0 => AbiLogLevel::Trace,
            1 => AbiLogLevel::Debug,
            2 => AbiLogLevel::Info,
            3 => AbiLogLevel::Warn,
            4 => AbiLogLevel::Error,
            _ => AbiLogLevel::Error,
        }
    }
}

impl From<Level> for AbiLogLevel {
    fn from(value: Level) -> Self {
        match value {
            Level::TRACE => AbiLogLevel::Trace,
            Level::DEBUG => AbiLogLevel::Debug,
            Level::INFO => AbiLogLevel::Info,
            Level::WARN => AbiLogLevel::Warn,
            Level::ERROR => AbiLogLevel::Error,
        }
    }
}

impl From<AbiLogLevel> for Level {
    fn from(value: AbiLogLevel) -> Self {
        match value {
            AbiLogLevel::Trace => Level::TRACE,
            AbiLogLevel::Debug => Level::DEBUG,
            AbiLogLevel::Info => Level::INFO,
            AbiLogLevel::Warn => Level::WARN,
            AbiLogLevel::Error => Level::ERROR,
        }
    }
}

pub struct MakeAbiLogWriter;

impl<'a> MakeWriter<'a> for MakeAbiLogWriter {
    type Writer = ConsoleWriter;

    fn make_writer(&'a self) -> Self::Writer {
        ConsoleWriter {
            buffer: vec![],
            level: Level::TRACE,
        }
    }

    fn make_writer_for(&'a self, meta: &tracing::Metadata<'_>) -> Self::Writer {
        let level = *meta.level();
        ConsoleWriter {
            buffer: vec![],
            level,
        }
    }
}

pub struct ConsoleWriter {
    buffer: Vec<u8>,
    level: Level,
}

impl Write for ConsoleWriter {
    fn write(&mut self, buf: &[u8]) -> std::io::Result<usize> {
        self.buffer.write(buf)
    }

    fn flush(&mut self) -> std::io::Result<()> {
        Ok(())
    }
}

impl Drop for ConsoleWriter {
    fn drop(&mut self) {
        let mut len = self.buffer.len();
        if len > 0 && self.buffer[len - 1] == b'\n' {
            // Don't propagate the new line!
            len -= 1;
        }
        unsafe {
            _log(
                AbiLogLevel::from(self.level) as u32,
                self.buffer.as_ptr() as u32,
                len as u32,
            )
        }
    }
}

fn log_subscriber(level: AbiLogLevel) -> impl Subscriber + Send + Sync + 'static {
    let level = level.into();

    let fmt_layer = tracing_subscriber::fmt::layer()
        .with_ansi(false)
        .without_time()
        .with_thread_names(false)
        .with_thread_ids(false)
        .with_file(false)
        .with_line_number(false)
        .with_target(level == Level::TRACE)
        .with_level(false)
        .with_span_events(if level == Level::TRACE {
            FmtSpan::ENTER
        } else {
            FmtSpan::NONE
        })
        .with_writer(MakeAbiLogWriter)
        // We do filtering here too,
        // as it might get expensive to pass logs through
        // the various layers even though we don't need them
        .with_filter(LevelFilter::from_level(level));
    Registry::default().with(fmt_layer)
}

// --------- VM

pub struct WasmVM {
    vm: CoreVM,
}

pub struct WasmHeaders(Vec<pb::Header>);

impl HeaderMap for WasmHeaders {
    type Error = Infallible;

    fn extract(&self, name: &str) -> Result<Option<&str>, Self::Error> {
        for pb::Header { key, value } in &self.0 {
            if key.eq_ignore_ascii_case(name) {
                return Ok(Some(value));
            }
        }
        Ok(None)
    }
}

#[export_name = "vm_new"]
pub unsafe extern "C" fn _vm_new(ptr: *mut u8, len: usize) -> u64 {
    let input = ptr_to_input(ptr, len);
    let response = vm_new(input);
    output_to_ptr(response)
}

fn vm_new(input: pb::VmNewParameters) -> pb::VmNewReturn {
    pb::VmNewReturn {
        result: Some(
            match CoreVM::new(WasmHeaders(input.headers), VMOptions::default()) {
                Ok(vm) => {
                    let wasm_vm = WasmVM { vm };
                    pb::vm_new_return::Result::Pointer(
                        Rc::into_raw(Rc::new(RefCell::new(wasm_vm))) as u32
                    )
                }
                Err(e) => pb::vm_new_return::Result::Failure(e.into()),
            },
        ),
    }
}

#[export_name = "vm_get_response_head"]
pub unsafe extern "C" fn _vm_get_response_head(vm_pointer: *const RefCell<WasmVM>) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let response: pb::VmGetResponseHeadReturn = VM::get_response_head(&rc_vm.borrow().vm).into();
    output_to_ptr(response)
}

#[export_name = "vm_notify_input"]
pub unsafe extern "C" fn _vm_notify_input(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let input = ptr_to_vec(ptr, len);
    VM::notify_input(&mut rc_vm.borrow_mut().vm, input.into());
}

#[export_name = "vm_notify_input_closed"]
pub unsafe extern "C" fn _vm_notify_input_closed(vm_pointer: *const RefCell<WasmVM>) {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    VM::notify_input_closed(&mut rc_vm.borrow_mut().vm);
}

#[export_name = "vm_notify_error"]
pub unsafe extern "C" fn _vm_notify_error(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let input = ptr_to_input(ptr, len);
    vm_notify_error(&rc_vm, input);
}

fn vm_notify_error(rc_vm: &Rc<RefCell<WasmVM>>, input: pb::VmNotifyError) {
    VM::notify_error(
        &mut rc_vm.borrow_mut().vm,
        Error::new(500u16, Cow::Owned(input.message)).with_stacktrace(input.stacktrace),
        None,
    )
}

#[export_name = "vm_take_output"]
pub unsafe extern "C" fn _vm_take_output(vm_pointer: *const RefCell<WasmVM>) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let res: pb::VmTakeOutputReturn = VM::take_output(&mut rc_vm.borrow_mut().vm).into();
    output_to_ptr(res)
}

#[export_name = "vm_is_ready_to_execute"]
pub unsafe extern "C" fn _vm_is_ready_to_execute(vm_pointer: *const RefCell<WasmVM>) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let res = vm_is_ready_to_execute(&rc_vm);
    output_to_ptr(res)
}

fn vm_is_ready_to_execute(rc_vm: &Rc<RefCell<WasmVM>>) -> pb::VmIsReadyToExecuteReturn {
    pb::VmIsReadyToExecuteReturn {
        result: Some(match VM::is_ready_to_execute(&rc_vm.borrow().vm) {
            Ok(ready) => pb::vm_is_ready_to_execute_return::Result::Ready(ready),
            Err(e) => pb::vm_is_ready_to_execute_return::Result::Failure(e.into()),
        }),
    }
}

#[export_name = "vm_is_completed"]
pub unsafe extern "C" fn _vm_is_completed(vm_pointer: *const RefCell<WasmVM>, handle: u32) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let result = VM::is_completed(&rc_vm.borrow().vm, NotificationHandle::from(handle));
    result as u64
}

#[export_name = "vm_is_processing"]
pub unsafe extern "C" fn _vm_is_processing(vm_pointer: *const RefCell<WasmVM>) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let result = VM::is_processing(&rc_vm.borrow().vm);
    result as u64
}

#[export_name = "vm_do_progress"]
pub unsafe extern "C" fn _vm_do_progress(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let input = ptr_to_input(ptr, len);
    let res = vm_do_progress(&rc_vm, input);
    output_to_ptr(res)
}

fn vm_do_progress(
    rc_vm: &Rc<RefCell<WasmVM>>,
    input: pb::VmDoProgressParameters,
) -> pb::VmDoProgressReturn {
    pb::VmDoProgressReturn {
        result: Some(
            match VM::do_progress(
                &mut rc_vm.borrow_mut().vm,
                input
                    .handles
                    .into_iter()
                    .map(NotificationHandle::from)
                    .collect(),
            ) {
                Ok(DoProgressResponse::AnyCompleted) => {
                    pb::vm_do_progress_return::Result::AnyCompleted(Default::default())
                }
                Ok(DoProgressResponse::ReadFromInput) => {
                    pb::vm_do_progress_return::Result::ReadFromInput(Default::default())
                }
                Ok(DoProgressResponse::CancelSignalReceived) => {
                    pb::vm_do_progress_return::Result::CancelSignalReceived(Default::default())
                }
                Ok(DoProgressResponse::WaitingPendingRun) => {
                    pb::vm_do_progress_return::Result::WaitingPendingRun(Default::default())
                }
                Ok(DoProgressResponse::ExecuteRun(handle)) => {
                    pb::vm_do_progress_return::Result::ExecuteRun(handle.into())
                }
                Err(e) if e.is_suspended_error() => {
                    pb::vm_do_progress_return::Result::Suspended(Default::default())
                }
                Err(e) => pb::vm_do_progress_return::Result::Failure(e.into()),
            },
        ),
    }
}

#[export_name = "vm_take_notification"]
pub unsafe extern "C" fn _vm_take_notification(
    vm_pointer: *const RefCell<WasmVM>,
    handle: u32,
) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let res = vm_take_notification(&rc_vm, NotificationHandle::from(handle));
    output_to_ptr(res)
}

fn vm_take_notification(
    rc_vm: &Rc<RefCell<WasmVM>>,
    handle: NotificationHandle,
) -> pb::VmTakeNotificationReturn {
    pb::VmTakeNotificationReturn {
        result: Some(
            match VM::take_notification(&mut rc_vm.borrow_mut().vm, handle) {
                Ok(None) => pb::vm_take_notification_return::Result::NotReady(Default::default()),
                Ok(Some(v)) => pb::vm_take_notification_return::Result::Value(pb::Value {
                    value: Some(match v {
                        Value::Void => pb::value::Value::Void(Default::default()),
                        Value::Success(s) => pb::value::Value::Success(s),
                        Value::Failure(f) => pb::value::Value::Failure(f.into()),
                        Value::StateKeys(sk) => {
                            pb::value::Value::StateKeys(pb::value::StateKeys { keys: sk })
                        }
                        Value::InvocationId(s) => pb::value::Value::InvocationId(s),
                    }),
                }),
                Err(e) if e.is_suspended_error() => {
                    pb::vm_take_notification_return::Result::Suspended(Default::default())
                }
                Err(e) => pb::vm_take_notification_return::Result::Failure(e.into()),
            },
        ),
    }
}

#[export_name = "vm_sys_input"]
pub unsafe extern "C" fn _vm_sys_input(vm_pointer: *const RefCell<WasmVM>) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let res = vm_sys_input(&rc_vm);
    output_to_ptr(res)
}

fn vm_sys_input(rc_vm: &Rc<RefCell<WasmVM>>) -> pb::VmSysInputReturn {
    let mut vm = rc_vm.borrow_mut();
    let protocol_version = vm.vm.get_response_head().version;
    pb::VmSysInputReturn {
        result: Some(match VM::sys_input(&mut vm.vm) {
            Ok(input) => pb::vm_sys_input_return::Result::Ok(pb::vm_sys_input_return::Input {
                invocation_id: input.invocation_id,
                key: input.key,
                headers: input.headers.into_iter().map(Into::into).collect(),
                input: input.input,
                random_seed: input.random_seed,
                should_use_random_seed: protocol_version >= Version::V6,
            }),
            Err(e) => pb::vm_sys_input_return::Result::Failure(e.into()),
        }),
    }
}

#[export_name = "vm_sys_state_get"]
pub unsafe extern "C" fn _vm_sys_state_get(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let input = ptr_to_input(ptr, len);
    let res = vm_sys_state_get(&rc_vm, input);
    output_to_ptr(res)
}

fn vm_sys_state_get(
    rc_vm: &Rc<RefCell<WasmVM>>,
    input: pb::VmSysStateGetParameters,
) -> pb::SimpleSysAsyncResultReturn {
    VM::sys_state_get(
        &mut rc_vm.borrow_mut().vm,
        input.key,
        PayloadOptions {
            unstable_serialization: input.unstable_serialization,
        },
    )
    .into()
}

#[export_name = "vm_sys_state_get_keys"]
pub unsafe extern "C" fn _sys_vm_state_get_keys(vm_pointer: *const RefCell<WasmVM>) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let res = vm_state_get_keys(&rc_vm);
    output_to_ptr(res)
}

fn vm_state_get_keys(rc_vm: &Rc<RefCell<WasmVM>>) -> pb::SimpleSysAsyncResultReturn {
    VM::sys_state_get_keys(&mut rc_vm.borrow_mut().vm).into()
}

#[export_name = "vm_sys_state_set"]
pub unsafe extern "C" fn _vm_sys_state_set(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let input = ptr_to_input(ptr, len);
    let res = vm_sys_state_set(&rc_vm, input);
    output_to_ptr(res)
}

fn vm_sys_state_set(
    rc_vm: &Rc<RefCell<WasmVM>>,
    input: pb::VmSysStateSetParameters,
) -> pb::GenericEmptyReturn {
    VM::sys_state_set(
        &mut rc_vm.borrow_mut().vm,
        input.key,
        input.value,
        PayloadOptions {
            unstable_serialization: input.unstable_serialization,
        },
    )
    .into()
}

#[export_name = "vm_sys_state_clear"]
pub unsafe extern "C" fn _vm_sys_state_clear(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let input = ptr_to_input(ptr, len);
    let res = vm_sys_state_clear(&rc_vm, input);
    output_to_ptr(res)
}

fn vm_sys_state_clear(
    rc_vm: &Rc<RefCell<WasmVM>>,
    input: pb::VmSysStateClearParameters,
) -> pb::GenericEmptyReturn {
    VM::sys_state_clear(&mut rc_vm.borrow_mut().vm, input.key).into()
}

#[export_name = "vm_sys_state_clear_all"]
pub unsafe extern "C" fn _vm_sys_state_clear_all(vm_pointer: *const RefCell<WasmVM>) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let res = vm_sys_state_clear_all(&rc_vm);
    output_to_ptr(res)
}

fn vm_sys_state_clear_all(rc_vm: &Rc<RefCell<WasmVM>>) -> pb::GenericEmptyReturn {
    VM::sys_state_clear_all(&mut rc_vm.borrow_mut().vm).into()
}

#[export_name = "vm_sys_sleep"]
pub unsafe extern "C" fn _vm_sys_sleep(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let input = ptr_to_input(ptr, len);
    let res = vm_sys_sleep(&rc_vm, input);
    output_to_ptr(res)
}

fn vm_sys_sleep(
    rc_vm: &Rc<RefCell<WasmVM>>,
    input: pb::VmSysSleepParameters,
) -> pb::SimpleSysAsyncResultReturn {
    VM::sys_sleep(
        &mut rc_vm.borrow_mut().vm,
        input.name,
        Duration::from_millis(input.wake_up_time_since_unix_epoch_millis),
        Some(Duration::from_millis(input.now_since_unix_epoch_millis)),
    )
    .into()
}

#[export_name = "vm_sys_awakeable"]
pub unsafe extern "C" fn _vm_sys_awakeable(vm_pointer: *const RefCell<WasmVM>) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let res = vm_sys_awakeable(&rc_vm);
    output_to_ptr(res)
}

fn vm_sys_awakeable(rc_vm: &Rc<RefCell<WasmVM>>) -> pb::VmSysAwakeableReturn {
    pb::VmSysAwakeableReturn {
        result: Some(match VM::sys_awakeable(&mut rc_vm.borrow_mut().vm) {
            Ok((awakeable_id, handle)) => {
                pb::vm_sys_awakeable_return::Result::Ok(pb::vm_sys_awakeable_return::Awakeable {
                    id: awakeable_id,
                    handle: handle.into(),
                })
            }
            Err(e) => pb::vm_sys_awakeable_return::Result::Failure(e.into()),
        }),
    }
}

#[export_name = "vm_sys_complete_awakeable"]
pub unsafe extern "C" fn _vm_sys_complete_awakeable(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let input = ptr_to_input(ptr, len);
    let res = vm_sys_complete_awakeable(&rc_vm, input);
    output_to_ptr(res)
}

fn vm_sys_complete_awakeable(
    rc_vm: &Rc<RefCell<WasmVM>>,
    input: pb::VmSysCompleteAwakeableParameters,
) -> pb::GenericEmptyReturn {
    VM::sys_complete_awakeable(
        &mut rc_vm.borrow_mut().vm,
        input.id,
        match input.result.expect("result should be present") {
            pb::vm_sys_complete_awakeable_parameters::Result::Success(s) => {
                NonEmptyValue::Success(s)
            }
            pb::vm_sys_complete_awakeable_parameters::Result::Failure(f) => {
                NonEmptyValue::Failure(f.into())
            }
        },
        PayloadOptions {
            unstable_serialization: input.unstable_serialization,
        },
    )
    .into()
}

#[export_name = "vm_sys_call"]
pub unsafe extern "C" fn _vm_sys_call(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let input = ptr_to_input(ptr, len);
    let res = vm_sys_call(&rc_vm, input);
    output_to_ptr(res)
}

fn vm_sys_call(rc_vm: &Rc<RefCell<WasmVM>>, input: pb::VmSysCallParameters) -> pb::VmSysCallReturn {
    pb::VmSysCallReturn {
        result: Some(
            match VM::sys_call(
                &mut rc_vm.borrow_mut().vm,
                Target {
                    service: input.service,
                    handler: input.handler,
                    key: input.key,
                    idempotency_key: input.idempotency_key,
                    headers: input.headers.into_iter().map(Into::into).collect(),
                },
                input.input,
                PayloadOptions {
                    unstable_serialization: input.unstable_serialization,
                },
            ) {
                Ok(call_handle) => {
                    pb::vm_sys_call_return::Result::Ok(pb::vm_sys_call_return::CallHandles {
                        invocation_id_handle: call_handle.invocation_id_notification_handle.into(),
                        result_handle: call_handle.call_notification_handle.into(),
                    })
                }
                Err(e) => pb::vm_sys_call_return::Result::Failure(e.into()),
            },
        ),
    }
}

#[export_name = "vm_sys_send"]
pub unsafe extern "C" fn _vm_sys_send(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let input = ptr_to_input(ptr, len);
    let res = vm_sys_send(&rc_vm, input);
    output_to_ptr(res)
}

fn vm_sys_send(
    rc_vm: &Rc<RefCell<WasmVM>>,
    input: pb::VmSysSendParameters,
) -> pb::SimpleSysAsyncResultReturn {
    VM::sys_send(
        &mut rc_vm.borrow_mut().vm,
        Target {
            service: input.service,
            handler: input.handler,
            key: input.key,
            idempotency_key: input.idempotency_key,
            headers: input.headers.into_iter().map(Into::into).collect(),
        },
        input.input,
        input
            .execution_time_since_unix_epoch_millis
            .map(Duration::from_millis),
        PayloadOptions {
            unstable_serialization: input.unstable_serialization,
        },
    )
    .map(|s| s.invocation_id_notification_handle)
    .into()
}

#[export_name = "vm_sys_cancel_invocation"]
pub unsafe extern "C" fn _vm_sys_cancel_invocation(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let input = ptr_to_input(ptr, len);
    let res = vm_sys_cancel_invocation(&rc_vm, input);
    output_to_ptr(res)
}

fn vm_sys_cancel_invocation(
    rc_vm: &Rc<RefCell<WasmVM>>,
    input: pb::VmSysCancelInvocation,
) -> pb::GenericEmptyReturn {
    VM::sys_cancel_invocation(&mut rc_vm.borrow_mut().vm, input.invocation_id).into()
}

#[export_name = "vm_sys_attach_invocation"]
pub unsafe extern "C" fn _vm_sys_attach_invocation(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let input = ptr_to_input(ptr, len);
    let res = vm_sys_attach_invocation(&rc_vm, input);
    output_to_ptr(res)
}

fn vm_sys_attach_invocation(
    rc_vm: &Rc<RefCell<WasmVM>>,
    input: pb::VmSysAttachInvocation,
) -> pb::SimpleSysAsyncResultReturn {
    VM::sys_attach_invocation(
        &mut rc_vm.borrow_mut().vm,
        AttachInvocationTarget::InvocationId(input.invocation_id),
    )
    .into()
}

#[export_name = "vm_sys_promise_get"]
pub unsafe extern "C" fn _vm_sys_promise_get(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let input = ptr_to_input(ptr, len);
    let res = vm_sys_promise_get(&rc_vm, input);
    output_to_ptr(res)
}

fn vm_sys_promise_get(
    rc_vm: &Rc<RefCell<WasmVM>>,
    input: pb::VmSysPromiseGetParameters,
) -> pb::SimpleSysAsyncResultReturn {
    VM::sys_get_promise(&mut rc_vm.borrow_mut().vm, input.key).into()
}

#[export_name = "vm_sys_promise_peek"]
pub unsafe extern "C" fn _vm_sys_promise_peek(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let input = ptr_to_input(ptr, len);
    let res = vm_sys_promise_peek(&rc_vm, input);
    output_to_ptr(res)
}

fn vm_sys_promise_peek(
    rc_vm: &Rc<RefCell<WasmVM>>,
    input: pb::VmSysPromisePeekParameters,
) -> pb::SimpleSysAsyncResultReturn {
    VM::sys_peek_promise(&mut rc_vm.borrow_mut().vm, input.key).into()
}

#[export_name = "vm_sys_promise_complete"]
pub unsafe extern "C" fn _vm_sys_promise_complete(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let input = ptr_to_input(ptr, len);
    let res = vm_sys_promise_complete(&rc_vm, input);
    output_to_ptr(res)
}

fn vm_sys_promise_complete(
    rc_vm: &Rc<RefCell<WasmVM>>,
    input: pb::VmSysPromiseCompleteParameters,
) -> pb::SimpleSysAsyncResultReturn {
    VM::sys_complete_promise(
        &mut rc_vm.borrow_mut().vm,
        input.id,
        match input.result.expect("result should be present") {
            pb::vm_sys_promise_complete_parameters::Result::Success(s) => NonEmptyValue::Success(s),
            pb::vm_sys_promise_complete_parameters::Result::Failure(f) => {
                NonEmptyValue::Failure(f.into())
            }
        },
        PayloadOptions {
            unstable_serialization: input.unstable_serialization,
        },
    )
    .into()
}

#[export_name = "vm_sys_run"]
pub unsafe extern "C" fn _vm_sys_run(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let input = ptr_to_input(ptr, len);
    let res = vm_sys_run(&rc_vm, input);
    output_to_ptr(res)
}

fn vm_sys_run(
    rc_vm: &Rc<RefCell<WasmVM>>,
    input: pb::VmSysRunParameters,
) -> pb::SimpleSysAsyncResultReturn {
    VM::sys_run(&mut rc_vm.borrow_mut().vm, input.name).into()
}

#[export_name = "vm_propose_run_completion"]
pub unsafe extern "C" fn _vm_propose_run_completion(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let input = ptr_to_input(ptr, len);
    let res = vm_propose_run_completion(&rc_vm, input);
    output_to_ptr(res)
}

fn vm_propose_run_completion(
    rc_vm: &Rc<RefCell<WasmVM>>,
    input: pb::VmProposeRunCompletionParameters,
) -> pb::GenericEmptyReturn {
    let run_exit_result = match input.result.expect("result must be there") {
        pb::vm_propose_run_completion_parameters::Result::Success(s) => RunExitResult::Success(s),
        pb::vm_propose_run_completion_parameters::Result::TerminalFailure(f) => {
            RunExitResult::TerminalFailure(f.into())
        }
        pb::vm_propose_run_completion_parameters::Result::RetryableFailure(f) => {
            RunExitResult::RetryableFailure {
                attempt_duration: Duration::from_millis(input.attempt_duration_millis),
                error: f.into(),
            }
        }
    };

    let retry_policy = match input.retry_policy {
        None => RetryPolicy::default(),
        Some(rp) => RetryPolicy::Exponential {
            initial_interval: Duration::from_millis(rp.initial_internal_millis),
            factor: rp.factor,
            max_interval: rp.max_interval_millis.map(Duration::from_millis),
            max_attempts: rp.max_attempts,
            max_duration: rp.max_duration_millis.map(Duration::from_millis),
        },
    };

    VM::propose_run_completion(
        &mut rc_vm.borrow_mut().vm,
        input.handle.into(),
        run_exit_result,
        retry_policy,
    )
    .into()
}

#[export_name = "vm_sys_write_output"]
pub unsafe extern "C" fn _vm_sys_write_output(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let input = ptr_to_input(ptr, len);
    let res = vm_sys_write_output(&rc_vm, input);
    output_to_ptr(res)
}

fn vm_sys_write_output(
    rc_vm: &Rc<RefCell<WasmVM>>,
    input: pb::VmSysWriteOutputParameters,
) -> pb::GenericEmptyReturn {
    let value = match input.result.expect("Should be present") {
        pb::vm_sys_write_output_parameters::Result::Success(s) => NonEmptyValue::Success(s),
        pb::vm_sys_write_output_parameters::Result::Failure(f) => NonEmptyValue::Failure(f.into()),
    };
    VM::sys_write_output(
        &mut rc_vm.borrow_mut().vm,
        value,
        PayloadOptions {
            unstable_serialization: input.unstable_serialization,
        },
    )
    .into()
}

#[export_name = "vm_sys_end"]
pub unsafe extern "C" fn _vm_sys_end(vm_pointer: *const RefCell<WasmVM>) -> u64 {
    let rc_vm = vm_ptr_to_rc(vm_pointer);
    let res = vm_sys_end(&rc_vm);
    output_to_ptr(res)
}

fn vm_sys_end(rc_vm: &Rc<RefCell<WasmVM>>) -> pb::GenericEmptyReturn {
    VM::sys_end(&mut rc_vm.borrow_mut().vm).into()
}

#[export_name = "vm_free"]
pub unsafe extern "C" fn _vm_free(vm: *const RefCell<WasmVM>) {
    assert_not_null(vm);
    unsafe {
        // We don't need to increment the counter, we're materializing the initial leak!
        let rc = Rc::from_raw(vm);
        match Rc::try_unwrap(rc) {
            Ok(cell) => cell.into_inner(),
            Err(_) => panic!("attempted to free vm while still borrowed"),
        }
    };
}

// --------- Logging infra

/// Logs a message to the console using [`_log`].
fn log(level: AbiLogLevel, message: &str) {
    unsafe {
        let (ptr, len) = string_to_ptr(message);
        _log(level as u32, ptr, len);
    }
}

#[link(wasm_import_module = "env")]
extern "C" {
    /// WebAssembly import which prints a string (linear memory offset,
    /// byteCount) to the console.
    ///
    /// Note: This is not an ownership transfer: Rust still owns the pointer
    /// and ensures it isn't deallocated during this call.
    #[link_name = "log"]
    fn _log(level: u32, ptr: u32, size: u32);
}

// --------- Unsafe stuff

#[inline]
pub fn assert_not_null<T>(s: *const T) {
    if s.is_null() {
        panic!("Null pointer exception on input")
    }
}

#[inline]
unsafe fn ptr_to_vec(ptr: *mut u8, len: usize) -> Vec<u8> {
    assert_not_null(ptr);
    Vec::from_raw_parts(ptr, len, len)
}

#[inline]
unsafe fn ptr_to_input<T: prost::Message + Default>(ptr: *mut u8, len: usize) -> T {
    let vec = ptr_to_vec(ptr, len);
    T::decode(&*vec).expect("Deserialization of golang payloads should not fail")
}

#[inline]
unsafe fn vec_to_ptr(v: Vec<u8>) -> u64 {
    let len = v.len();
    let ptr = Box::into_raw(v.into_boxed_slice()) as *mut u8;
    ((ptr as u64) << 32) | len as u64
}

#[inline]
unsafe fn output_to_ptr<T: prost::Message>(t: T) -> u64 {
    vec_to_ptr(t.encode_to_vec())
}

unsafe fn vm_ptr_to_rc(vm_pointer: *const RefCell<WasmVM>) -> Rc<RefCell<WasmVM>> {
    assert_not_null(vm_pointer);
    Rc::increment_strong_count(vm_pointer);
    Rc::from_raw(vm_pointer)
}

/// Returns a pointer and size pair for the given string in a way compatible
/// with WebAssembly numeric types.
///
/// Note: This doesn't change the ownership of the String. To intentionally
/// leak it, use [`std::mem::forget`] on the input after calling this.
unsafe fn string_to_ptr(s: &str) -> (u32, u32) {
    (s.as_ptr() as u32, s.len() as u32)
}

/// WebAssembly export that allocates a pointer (linear memory offset) that can
/// be used for a string.
///
/// This is an ownership transfer, which means the caller must call
/// [`deallocate`] when finished.
#[export_name = "allocate"]
pub unsafe extern "C" fn _allocate(size: usize) -> *mut u8 {
    allocate(size)
}

/// Allocates size bytes and leaks the pointer where they start.
fn allocate(size: usize) -> *mut u8 {
    // Allocate the amount of bytes needed.
    let vec: Vec<MaybeUninit<u8>> = vec![MaybeUninit::uninit(); size];
    // into_raw leaks the memory to the caller.
    Box::into_raw(vec.into_boxed_slice()) as *mut u8
}

/// WebAssembly export that deallocates a pointer of the given size (linear
/// memory offset, byteCount) allocated by [`allocate`].
#[export_name = "deallocate"]
pub unsafe extern "C" fn _deallocate(ptr: *mut u8, size: usize) {
    deallocate(ptr, size);
}

/// Retakes the pointer which allows its memory to be freed.
unsafe fn deallocate(ptr: *mut u8, size: usize) {
    let _: Vec<u8> = Vec::from_raw_parts(ptr, 0, size);
}

// -- Data structures

mod pb {
    include!(concat!(env!("OUT_DIR"), "/_.rs"));
}

impl From<Error> for pb::Failure {
    fn from(value: Error) -> Self {
        Self {
            code: value.code().into(),
            message: value.to_string(),
        }
    }
}

impl From<pb::Failure> for Error {
    fn from(value: pb::Failure) -> Self {
        Self::new(value.code as u16, value.message)
    }
}

impl From<pb::FailureWithStacktrace> for Error {
    fn from(value: pb::FailureWithStacktrace) -> Self {
        Self::new(value.code as u16, value.message).with_stacktrace(value.stacktrace)
    }
}

impl From<pb::Failure> for TerminalFailure {
    fn from(value: pb::Failure) -> Self {
        Self {
            code: value.code.try_into().expect("Should be u16"),
            message: value.message,
            metadata: Vec::new(),
        }
    }
}

impl From<TerminalFailure> for pb::Failure {
    fn from(value: TerminalFailure) -> Self {
        Self {
            code: value.code.into(),
            message: value.message,
        }
    }
}

impl From<Header> for pb::Header {
    fn from(value: Header) -> Self {
        Self {
            key: value.key.into_owned(),
            value: value.value.into_owned(),
        }
    }
}

impl From<pb::Header> for Header {
    fn from(value: pb::Header) -> Self {
        Self {
            key: value.key.into(),
            value: value.value.into(),
        }
    }
}

impl From<TakeOutputResult> for pb::VmTakeOutputReturn {
    fn from(value: TakeOutputResult) -> Self {
        pb::VmTakeOutputReturn {
            result: Some(match value {
                TakeOutputResult::Buffer(b) => pb::vm_take_output_return::Result::Bytes(b),
                TakeOutputResult::EOF => pb::vm_take_output_return::Result::Eof(Default::default()),
            }),
        }
    }
}

impl From<Result<(), Error>> for pb::GenericEmptyReturn {
    fn from(value: Result<(), Error>) -> Self {
        Self {
            result: Some(match value {
                Ok(()) => pb::generic_empty_return::Result::Ok(Default::default()),
                Err(e) => pb::generic_empty_return::Result::Failure(e.into()),
            }),
        }
    }
}

impl From<Result<NotificationHandle, Error>> for pb::SimpleSysAsyncResultReturn {
    fn from(value: Result<NotificationHandle, Error>) -> Self {
        Self {
            result: Some(match value {
                Ok(handle) => pb::simple_sys_async_result_return::Result::Handle(handle.into()),
                Err(e) => pb::simple_sys_async_result_return::Result::Failure(e.into()),
            }),
        }
    }
}

impl From<ResponseHead> for pb::VmGetResponseHeadReturn {
    fn from(value: ResponseHead) -> Self {
        Self {
            status_code: value.status_code.into(),
            headers: value.headers.into_iter().map(Into::into).collect(),
        }
    }
}
