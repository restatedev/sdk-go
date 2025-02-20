extern crate alloc;
extern crate core;

use alloc::borrow::Cow;
use alloc::rc::Rc;
use alloc::vec::Vec;
use restate_sdk_shared_core::{
    CoreVM, DoProgressResponse, Error, Header, HeaderMap, Input, NonEmptyValue, NotificationHandle,
    ResponseHead, RetryPolicy, RunExitResult, SuspendedOrVMError, TakeOutputResult, Target,
    TerminalFailure, Value, VM,
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
pub extern "C" fn init() {
    std::panic::set_hook(Box::new(|panic| {
        let panic_str = format!("Core panicked: {panic}");
        log(&panic_str)
    }));
    // TODO change this, now to TRACE because IDK what's happening here
    let _ = tracing::subscriber::set_global_default(log_subscriber(LogLevel::TRACE, None));
}

#[repr(u32)]
pub enum LogLevel {
    TRACE = 0,
    DEBUG = 1,
    INFO = 2,
    WARN = 3,
    ERROR = 4,
}

pub struct MakeWebConsoleWriter {
    logger_id: Option<u32>,
}

impl<'a> MakeWriter<'a> for MakeWebConsoleWriter {
    type Writer = ConsoleWriter;

    fn make_writer(&'a self) -> Self::Writer {
        ConsoleWriter {
            buffer: vec![],
            level: Level::TRACE, // if no level is known, assume the most detailed
            logger_id: self.logger_id,
        }
    }

    fn make_writer_for(&'a self, meta: &tracing::Metadata<'_>) -> Self::Writer {
        let level = *meta.level();
        ConsoleWriter {
            buffer: vec![],
            level,
            logger_id: self.logger_id,
        }
    }
}

pub struct ConsoleWriter {
    buffer: Vec<u8>,

    level: Level,
    logger_id: Option<u32>,
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
        unsafe { _log(self.buffer.as_ptr() as u32, self.buffer.len() as u32) }
    }
}

fn log_subscriber(
    level: LogLevel,
    logger_id: Option<u32>,
) -> impl Subscriber + Send + Sync + 'static {
    let level = match level {
        LogLevel::TRACE => Level::TRACE,
        LogLevel::DEBUG => Level::DEBUG,
        LogLevel::INFO => Level::INFO,
        LogLevel::WARN => Level::WARN,
        LogLevel::ERROR => Level::ERROR,
    };

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
        .with_writer(MakeWebConsoleWriter { logger_id })
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
pub extern "C" fn _vm_new(ptr: *mut u8, len: usize) -> u64 {
    let input: pb::VmNewParameters = unsafe { ptr_to_input(ptr, len) };

    let response = pb::VmNewReturn {
        result: Some(
            match CoreVM::new(WasmHeaders(input.headers), Default::default()) {
                Ok(vm) => {
                    let wasm_vm = WasmVM { vm };
                    pb::vm_new_return::Result::Pointer(
                        Rc::into_raw(Rc::new(RefCell::new(wasm_vm))) as u32
                    )
                }
                Err(e) => pb::vm_new_return::Result::Failure(e.into()),
            },
        ),
    };

    unsafe { output_to_ptr(response) }
}

impl From<ResponseHead> for pb::VmGetResponseHeadReturn {
    fn from(value: ResponseHead) -> Self {
        Self {
            status_code: value.status_code.try_into().expect("should be u16"),
            headers: value.headers.into_iter().map(Into::into).collect(),
        }
    }
}

#[export_name = "vm_get_response_head"]
pub extern "C" fn _vm_get_response_head(vm_pointer: *const RefCell<WasmVM>) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };

    let response: pb::VmGetResponseHeadReturn = VM::get_response_head(&rc_vm.borrow().vm).into();

    unsafe { output_to_ptr(response) }
}

#[export_name = "vm_notify_input"]
pub extern "C" fn _vm_notify_input(vm_pointer: *const RefCell<WasmVM>, ptr: *mut u8, len: usize) {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };
    let input = unsafe { ptr_to_vec(ptr, len) };

    VM::notify_input(&mut rc_vm.borrow_mut().vm, input.into());
}

#[export_name = "vm_notify_input_closed"]
pub extern "C" fn _vm_notify_input_closed(vm_pointer: *const RefCell<WasmVM>) {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };

    VM::notify_input_closed(&mut rc_vm.borrow_mut().vm);
}

#[export_name = "vm_notify_error"]
pub extern "C" fn _vm_notify_error(vm_pointer: *const RefCell<WasmVM>, ptr: *mut u8, len: usize) {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };
    let input =
        unsafe { String::from_utf8(ptr_to_vec(ptr, len)).expect("String should be valid UTF-8") };

    VM::notify_error(
        &mut rc_vm.borrow_mut().vm,
        Error::new(500u16, Cow::Owned(input)),
        None,
    );
}

#[export_name = "vm_take_output"]
pub extern "C" fn _vm_take_output(vm_pointer: *const RefCell<WasmVM>) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };

    let res: pb::VmTakeOutputReturn = VM::take_output(&mut rc_vm.borrow_mut().vm).into();

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_is_ready_to_execute"]
pub extern "C" fn _vm_is_ready_to_execute(vm_pointer: *const RefCell<WasmVM>) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };

    let res = pb::VmIsReadyToExecuteReturn {
        result: Some(match VM::is_ready_to_execute(&rc_vm.borrow().vm) {
            Ok(ready) => pb::vm_is_ready_to_execute_return::Result::Ready(ready),
            Err(e) => pb::vm_is_ready_to_execute_return::Result::Failure(e.into()),
        }),
    };

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_is_completed"]
pub extern "C" fn _vm_is_completed(vm_pointer: *const RefCell<WasmVM>, handle: u32) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };

    let result = VM::is_completed(&rc_vm.borrow().vm, NotificationHandle::from(handle));
    result as u64
}

#[export_name = "vm_do_progress"]
pub extern "C" fn _vm_do_progress(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };
    let input: pb::VmDoProgressParameters = unsafe { ptr_to_input(ptr, len) };

    let res = pb::VmDoProgressReturn {
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
                Err(SuspendedOrVMError::Suspended(_)) => {
                    pb::vm_do_progress_return::Result::Suspended(Default::default())
                }
                Err(SuspendedOrVMError::VM(e)) => {
                    pb::vm_do_progress_return::Result::Failure(e.into())
                }
            },
        ),
    };

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_take_notification"]
pub extern "C" fn _vm_take_notification(vm_pointer: *const RefCell<WasmVM>, handle: u32) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };
    let handle = NotificationHandle::from(handle);

    let res = pb::VmTakeNotificationReturn {
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
                Err(SuspendedOrVMError::Suspended(_)) => {
                    pb::vm_take_notification_return::Result::Suspended(Default::default())
                }
                Err(SuspendedOrVMError::VM(e)) => {
                    pb::vm_take_notification_return::Result::Failure(e.into())
                }
            },
        ),
    };

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_sys_input"]
pub extern "C" fn _vm_sys_input(vm_pointer: *const RefCell<WasmVM>) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };

    let res = pb::VmSysInputReturn {
        result: Some(match VM::sys_input(&mut rc_vm.borrow_mut().vm) {
            Ok(input) => pb::vm_sys_input_return::Result::Ok(input.into()),
            Err(e) => pb::vm_sys_input_return::Result::Failure(e.into()),
        }),
    };

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_sys_state_get"]
pub extern "C" fn _vm_sys_state_get(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };
    let input: pb::VmSysStateGetParameters = unsafe { ptr_to_input(ptr, len) };

    let res: pb::SimpleSysAsyncResultReturn =
        VM::sys_state_get(&mut rc_vm.borrow_mut().vm, input.key).into();

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_sys_state_get_keys"]
pub extern "C" fn _sys_vm_state_get_keys(vm_pointer: *const RefCell<WasmVM>) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };

    let res: pb::SimpleSysAsyncResultReturn =
        VM::sys_state_get_keys(&mut rc_vm.borrow_mut().vm).into();

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_sys_state_set"]
pub extern "C" fn _vm_sys_state_set(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };
    let input: pb::VmSysStateSetParameters = unsafe { ptr_to_input(ptr, len) };

    let res: pb::GenericEmptyReturn =
        VM::sys_state_set(&mut rc_vm.borrow_mut().vm, input.key, input.value).into();

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_sys_state_clear"]
pub extern "C" fn _vm_sys_state_clear(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };
    let input: pb::VmSysStateClearParameters = unsafe { ptr_to_input(ptr, len) };

    let res: pb::GenericEmptyReturn =
        VM::sys_state_clear(&mut rc_vm.borrow_mut().vm, input.key).into();

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_sys_state_clear_all"]
pub extern "C" fn _vm_sys_state_clear_all(vm_pointer: *const RefCell<WasmVM>) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };

    let res: pb::GenericEmptyReturn = VM::sys_state_clear_all(&mut rc_vm.borrow_mut().vm).into();

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_sys_sleep"]
pub extern "C" fn _vm_sys_sleep(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };
    let input: pb::VmSysSleepParameters = unsafe { ptr_to_input(ptr, len) };

    let res: pb::SimpleSysAsyncResultReturn = VM::sys_sleep(
        &mut rc_vm.borrow_mut().vm,
        Duration::from_millis(input.wake_up_time_since_unix_epoch_millis),
        Some(Duration::from_millis(input.now_since_unix_epoch_millis)),
    )
    .into();

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_sys_awakeable"]
pub extern "C" fn _vm_sys_awakeable(vm_pointer: *const RefCell<WasmVM>) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };

    let res = pb::VmSysAwakeableReturn {
        result: Some(match VM::sys_awakeable(&mut rc_vm.borrow_mut().vm) {
            Ok((awakeable_id, handle)) => {
                pb::vm_sys_awakeable_return::Result::Ok(pb::vm_sys_awakeable_return::Awakeable {
                    id: awakeable_id,
                    handle: handle.into(),
                })
            }
            Err(e) => pb::vm_sys_awakeable_return::Result::Failure(e.into()),
        }),
    };

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_sys_complete_awakeable"]
pub extern "C" fn _vm_sys_complete_awakeable(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };
    let input: pb::VmSysCompleteAwakeableParameters = unsafe { ptr_to_input(ptr, len) };

    let res: pb::GenericEmptyReturn = VM::sys_complete_awakeable(
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
    )
    .into();

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_sys_call"]
pub extern "C" fn _vm_sys_call(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };
    let input: pb::VmSysCallParameters = unsafe { ptr_to_input(ptr, len) };

    let res = pb::VmSysCallReturn {
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
    };

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_sys_send"]
pub extern "C" fn _vm_sys_send(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };
    let input: pb::VmSysSendParameters = unsafe { ptr_to_input(ptr, len) };

    let res: pb::SimpleSysAsyncResultReturn = VM::sys_send(
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
    )
    .map(|s| s.invocation_id_notification_handle)
    .into();

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_sys_promise_get"]
pub extern "C" fn _vm_sys_promise_get(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };
    let input: pb::VmSysPromiseGetParameters = unsafe { ptr_to_input(ptr, len) };

    let res: pb::SimpleSysAsyncResultReturn =
        VM::sys_get_promise(&mut rc_vm.borrow_mut().vm, input.key).into();

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_sys_promise_peek"]
pub extern "C" fn _vm_sys_promise_peek(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };
    let input: pb::VmSysPromisePeekParameters = unsafe { ptr_to_input(ptr, len) };

    let res: pb::SimpleSysAsyncResultReturn =
        VM::sys_peek_promise(&mut rc_vm.borrow_mut().vm, input.key).into();

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_sys_promise_complete"]
pub extern "C" fn _vm_sys_promise_complete(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };
    let input: pb::VmSysPromiseCompleteParameters = unsafe { ptr_to_input(ptr, len) };

    let res: pb::SimpleSysAsyncResultReturn = VM::sys_complete_promise(
        &mut rc_vm.borrow_mut().vm,
        input.id,
        match input.result.expect("result should be present") {
            pb::vm_sys_promise_complete_parameters::Result::Success(s) => NonEmptyValue::Success(s),
            pb::vm_sys_promise_complete_parameters::Result::Failure(f) => {
                NonEmptyValue::Failure(f.into())
            }
        },
    )
    .into();

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_sys_run"]
pub extern "C" fn _vm_sys_run(vm_pointer: *const RefCell<WasmVM>, ptr: *mut u8, len: usize) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };
    let input: pb::VmSysRunParameters = unsafe { ptr_to_input(ptr, len) };

    let res: pb::SimpleSysAsyncResultReturn =
        VM::sys_run(&mut rc_vm.borrow_mut().vm, input.name).into();

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_propose_run_completion"]
pub extern "C" fn _vm_propose_run_completion(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };
    let input: pb::VmProposeRunCompletionParameters = unsafe { ptr_to_input(ptr, len) };

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
        None => RetryPolicy::None,
        Some(rp) => RetryPolicy::Exponential {
            initial_interval: Duration::from_millis(rp.initial_internal_millis),
            factor: rp.factor,
            max_interval: rp.max_interval_millis.map(Duration::from_millis),
            max_attempts: rp.max_attempts,
            max_duration: rp.max_duration_millis.map(Duration::from_millis),
        },
    };

    let res: pb::GenericEmptyReturn = VM::propose_run_completion(
        &mut rc_vm.borrow_mut().vm,
        input.handle.into(),
        run_exit_result,
        retry_policy,
    )
    .into();

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_sys_write_output"]
pub extern "C" fn _vm_write_output(
    vm_pointer: *const RefCell<WasmVM>,
    ptr: *mut u8,
    len: usize,
) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };
    let input: pb::VmSysWriteOutputParameters = unsafe { ptr_to_input(ptr, len) };

    let res: pb::GenericEmptyReturn =
        VM::sys_write_output(&mut rc_vm.borrow_mut().vm, input.into()).into();

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_sys_end"]
pub extern "C" fn _vm_sys_end(vm_pointer: *const RefCell<WasmVM>) -> u64 {
    let rc_vm = unsafe { vm_ptr_to_rc(vm_pointer) };

    let res: pb::GenericEmptyReturn = VM::sys_end(&mut rc_vm.borrow_mut().vm).into();

    unsafe { output_to_ptr(res) }
}

#[export_name = "vm_free"]
pub extern "C" fn _vm_free(vm: *const RefCell<WasmVM>) {
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
fn log(message: &str) {
    unsafe {
        let (ptr, len) = string_to_ptr(message);
        _log(ptr, len);
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
    fn _log(ptr: u32, size: u32);
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

/// Returns a string from WebAssembly compatible numeric types representing
/// its pointer and length.
// unsafe fn ptr_to_string(ptr: u32, len: u32) -> String {
//     let slice = slice::from_raw_parts_mut(ptr as *mut u8, len as usize);
//     let utf8 = std::str::from_utf8_unchecked_mut(slice);
//     return String::from(utf8);
// }

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
    deallocate(ptr as *mut u8, size as usize);
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
            code: value.code().try_into().expect("Should be u16"),
            message: value.to_string(),
        }
    }
}

impl From<pb::Failure> for Error {
    fn from(value: pb::Failure) -> Self {
        Self::new(value.code as u16, value.message)
    }
}

impl From<pb::Failure> for TerminalFailure {
    fn from(value: pb::Failure) -> Self {
        Self {
            code: value.code.try_into().expect("Should be u16"),
            message: value.message,
        }
    }
}

impl From<TerminalFailure> for pb::Failure {
    fn from(value: TerminalFailure) -> Self {
        Self {
            code: value.code.try_into().expect("Should be u16"),
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

impl From<Input> for pb::vm_sys_input_return::Input {
    fn from(value: Input) -> Self {
        Self {
            invocation_id: value.invocation_id,
            key: value.key,
            headers: value.headers.into_iter().map(Into::into).collect(),
            input: value.input,
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

impl From<pb::VmSysWriteOutputParameters> for NonEmptyValue {
    fn from(value: pb::VmSysWriteOutputParameters) -> Self {
        match value.result.expect("Should be present") {
            pb::vm_sys_write_output_parameters::Result::Success(s) => Self::Success(s),
            pb::vm_sys_write_output_parameters::Result::Failure(f) => Self::Failure(f.into()),
        }
    }
}
