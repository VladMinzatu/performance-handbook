//! The tokio server under study, on tokio's multi-thread runtime with hyper.
//!
//!   GET /cheap  returns a short response at once
//!   GET /heavy  runs a fixed amount of arithmetic, then responds
//!
//! The /heavy loop is the same as the Go server's, and its count is
//! calibrated the same way at startup, so both servers do the same work
//! per request.
//!
//! Knobs (env):
//!
//!   WORKERS     tokio worker threads (default 2)
//!   HEAVY_MS    target duration of one /heavy call on an idle core (default 200)
//!   HEAVY_MODE  inline:   run the loop in the handler, on the worker thread
//!               blocking: run it in tokio::task::spawn_blocking
//!               (default inline)

use std::convert::Infallible;
use std::hint::black_box;
use std::time::{Duration, Instant};

use bytes::Bytes;
use http_body_util::Full;
use hyper::body::Incoming;
use hyper::server::conn::http1;
use hyper::service::service_fn;
use hyper::{Request, Response, StatusCode};
use hyper_util::rt::TokioIo;
use tokio::net::TcpListener;

fn env_or<T: std::str::FromStr>(key: &str, def: T) -> T {
    std::env::var(key).ok().and_then(|v| v.parse().ok()).unwrap_or(def)
}

/// xorshift64 followed by a multiply, n times. Same loop as the Go server.
fn spin(n: u64) -> u64 {
    let mut x: u64 = 88172645463325252;
    for _ in 0..black_box(n) {
        x ^= x << 13;
        x ^= x >> 7;
        x ^= x << 17;
        x = x.wrapping_mul(0x2545F4914F6CDD1D);
    }
    black_box(x)
}

/// The loop count that takes about `target` on this core, using the
/// fastest of a few timed runs.
fn calibrate(target: Duration) -> u64 {
    const PROBE: u64 = 20_000_000;
    let best = (0..5)
        .map(|_| {
            let t0 = Instant::now();
            spin(PROBE);
            t0.elapsed()
        })
        .min()
        .unwrap();
    (PROBE as f64 * target.as_secs_f64() / best.as_secs_f64()) as u64
}

#[derive(Clone, Copy)]
enum Mode {
    Inline,
    Blocking,
}

async fn handle(req: Request<Incoming>, iters: u64, mode: Mode) -> Result<Response<Full<Bytes>>, Infallible> {
    let body = match req.uri().path() {
        "/cheap" => "ok\n".to_string(),
        "/heavy" => {
            let v = match mode {
                Mode::Inline => spin(iters),
                Mode::Blocking => tokio::task::spawn_blocking(move || spin(iters)).await.unwrap(),
            };
            format!("{v}\n")
        }
        _ => {
            let mut resp = Response::new(Full::new(Bytes::from_static(b"not found\n")));
            *resp.status_mut() = StatusCode::NOT_FOUND;
            return Ok(resp);
        }
    };
    Ok(Response::new(Full::new(Bytes::from(body))))
}

fn main() {
    let workers: usize = env_or("WORKERS", 2);
    let heavy_ms: u64 = env_or("HEAVY_MS", 200);
    let mode = match std::env::var("HEAVY_MODE").as_deref() {
        Ok("blocking") => Mode::Blocking,
        _ => Mode::Inline,
    };
    let iters = calibrate(Duration::from_millis(heavy_ms));
    eprintln!(
        "workers={workers} HEAVY_MS={heavy_ms} HEAVY_MODE={} iters={iters}",
        match mode {
            Mode::Inline => "inline",
            Mode::Blocking => "blocking",
        }
    );

    let rt = tokio::runtime::Builder::new_multi_thread()
        .worker_threads(workers)
        .enable_all()
        .build()
        .unwrap();

    rt.block_on(async move {
        let listener = TcpListener::bind("0.0.0.0:8080").await.unwrap();
        loop {
            let (stream, _) = match listener.accept().await {
                Ok(s) => s,
                Err(e) => {
                    eprintln!("accept: {e}");
                    continue;
                }
            };
            // Go's net/http sets TCP_NODELAY on every connection; match it.
            let _ = stream.set_nodelay(true);
            tokio::spawn(async move {
                let svc = service_fn(move |req| handle(req, iters, mode));
                let _ = http1::Builder::new().serve_connection(TokioIo::new(stream), svc).await;
            });
        }
    });
}
