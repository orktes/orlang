var Worker = require("worker-loader!./worker.js");

function createWorker(type) {
  let globalReject;
  let globalWorker = new Worker();
  return function Lint(src) {
    if (globalWorker) {
      globalReject && globalReject(new Error('Cancelled'));
      globalWorker.terminate();
    }
    globalWorker = new Worker();
    return new Promise((resolve, reject)=> {
      globalReject = reject;
      const ln = (event)=> {
        globalReject = null;
        globalWorker.terminate();
        globalWorker = null;

        const data = event.data;
        resolve(data.result);

      };
      globalWorker.addEventListener("message", ln);

      globalWorker.postMessage({
        type: type,
        code: src,
      })
    });
  }
}

export const Lint = createWorker("lint");
export const Compile = createWorker("compile");
export const Tokenize = createWorker("tokenize");
