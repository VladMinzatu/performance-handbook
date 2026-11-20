//! The tokio server under study: the same TCP push-gateway protocol as the
//! Go server. The client sends "SUB <topic>\n" and gets "OK\n"; after that
//! the connection only carries "PING\n" / "PONG\n" and is otherwise idle.
//!
//! MODE picks how each connection is handled:
//!
//!   pumps  a read task with a 4KB BufReader, and a write task draining an
//!          mpsc channel through a 4KB BufWriter (default). The sizes
//!          match Go's bufio rather than tokio's 8KB default.
//!   lean   one task waiting in readable().await. Once the socket has
//!          data, it reads into a buffer on the worker thread's stack and
//!          answers; the buffer isn't part of the task, so nothing is held
//!          while idle.
//!
//! Logs the size of each kind of task once, at the first connection, and
//! every 5s the number of connections.

use std::sync::Arc;
use std::sync::atomic::{AtomicI64, Ordering};
use std::time::{Duration, Instant};

use tokio::io::{AsyncBufReadExt, AsyncWriteExt, BufReader, BufWriter};
use tokio::net::{TcpListener, TcpStream};
use tokio::sync::mpsc;

const OK: &[u8] = b"OK\n";
const PONG: &[u8] = b"PONG\n";

fn reply(line: &[u8]) -> &'static [u8] {
    if line.starts_with(b"PING") { PONG } else { OK }
}

async fn write_pump(wr: tokio::net::tcp::OwnedWriteHalf, mut rx: mpsc::Receiver<&'static [u8]>) {
    let mut w = BufWriter::with_capacity(4096, wr);
    while let Some(msg) = rx.recv().await {
        if w.write_all(msg).await.is_err() || w.flush().await.is_err() {
            return;
        }
    }
}

async fn read_pump(rd: tokio::net::tcp::OwnedReadHalf, tx: mpsc::Sender<&'static [u8]>) {
    let mut r = BufReader::with_capacity(4096, rd);
    let mut line = Vec::new();
    loop {
        line.clear();
        match r.read_until(b'\n', &mut line).await {
            Ok(0) | Err(_) => return,
            Ok(_) => {
                if tx.send(reply(&line)).await.is_err() {
                    return;
                }
            }
        }
    }
}

async fn handle_lean(mut stream: TcpStream) {
    loop {
        if stream.readable().await.is_err() {
            return;
        }
        // The buffer lives only inside this block, which has no .await,
        // so it's on the worker thread's stack and not stored in the task.
        let (oks, pongs) = {
            let mut buf = [0u8; 4096];
            match stream.try_read(&mut buf) {
                Ok(0) => return,
                Ok(n) => {
                    let (mut oks, mut pongs) = (0, 0);
                    for line in buf[..n].split_inclusive(|&b| b == b'\n') {
                        if reply(line) == PONG { pongs += 1 } else { oks += 1 }
                    }
                    (oks, pongs)
                }
                Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => continue,
                Err(_) => return,
            }
        };
        for _ in 0..oks {
            if stream.write_all(OK).await.is_err() {
                return;
            }
        }
        for _ in 0..pongs {
            if stream.write_all(PONG).await.is_err() {
                return;
            }
        }
    }
}

fn spawn_pumps(stream: TcpStream, conns: Arc<AtomicI64>, log_sizes: bool) {
    let (rd, wr) = stream.into_split();
    let (tx, rx) = mpsc::channel(8);
    let writer = write_pump(wr, rx);
    let reader = read_pump(rd, tx);
    if log_sizes {
        eprintln!(
            "task sizes: read pump {}B, write pump {}B",
            std::mem::size_of_val(&reader),
            std::mem::size_of_val(&writer)
        );
    }
    tokio::spawn(writer);
    tokio::spawn(async move {
        reader.await;
        conns.fetch_sub(1, Ordering::Relaxed);
    });
}

fn spawn_lean(stream: TcpStream, conns: Arc<AtomicI64>, log_sizes: bool) {
    let task = handle_lean(stream);
    if log_sizes {
        eprintln!("task size: lean {}B", std::mem::size_of_val(&task));
    }
    tokio::spawn(async move {
        task.await;
        conns.fetch_sub(1, Ordering::Relaxed);
    });
}

fn main() {
    let workers: usize = std::env::var("WORKERS").ok().and_then(|v| v.parse().ok()).unwrap_or(2);
    let mode = std::env::var("MODE").unwrap_or_else(|_| "pumps".into());
    let spawn: fn(TcpStream, Arc<AtomicI64>, bool) = match mode.as_str() {
        "pumps" => spawn_pumps,
        "lean" => spawn_lean,
        m => panic!("unknown MODE {m:?} (want pumps or lean)"),
    };

    let rt = tokio::runtime::Builder::new_multi_thread()
        .worker_threads(workers)
        .enable_all()
        .build()
        .unwrap();

    rt.block_on(async move {
        let listener = TcpListener::bind("0.0.0.0:9000").await.unwrap();
        eprintln!("MODE={mode} workers={workers} listening on :9000");
        let conns = Arc::new(AtomicI64::new(0));

        let c = conns.clone();
        tokio::spawn(async move {
            let start = Instant::now();
            let mut tick = tokio::time::interval(Duration::from_secs(5));
            tick.tick().await;
            loop {
                tick.tick().await;
                println!("t={:4.0}s conns={:6}", start.elapsed().as_secs_f64(), c.load(Ordering::Relaxed));
            }
        });

        let mut first = true;
        loop {
            let stream = match listener.accept().await {
                Ok((s, _)) => s,
                Err(e) => {
                    eprintln!("accept: {e}");
                    tokio::time::sleep(Duration::from_millis(10)).await;
                    continue;
                }
            };
            let _ = stream.set_nodelay(true);
            conns.fetch_add(1, Ordering::Relaxed);
            spawn(stream, conns.clone(), first);
            first = false;
        }
    });
}
