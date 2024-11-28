async function getLCP(go) {
  const WASM_URL = "lcp.wasm";

  let instance;

  if ("instantiateStreaming" in WebAssembly) {
    instance = (
      await WebAssembly.instantiateStreaming(fetch(WASM_URL), go.importObject)
    ).instance;
  } else {
    const res = await fetch(WASM_URL);
    if (!res.ok) throw new Error(`unexpected status code: ${res.status}`);
    instance = (
      await WebAssembly.instantiate(await res.arrayBuffer(), go.importObject)
    ).instance;
  }

  go.run(instance);

  return instance;
}

/**
 * @param {WebAssembly.Instance} lcp
 * @param {string} s
 * @returns {number} The memory address
 */
function newGoString(lcp, s) {
  const data = new TextEncoder().encode(s);
  return newGoBytes(lcp, data);
}

/**
 * @param {WebAssembly.Instance} lcp
 * @param {Uint8Array} data
 * @returns {number} The memory address
 */
function newGoBytes(lcp, data) {
  const { newBytes } = lcp.exports;
  const addr = newBytes(data.length);
  // Read *after* calling newBytes
  const u8mem = new Uint8Array(lcp.exports.memory.buffer);

  for (let i = 0; i < data.length; ++i) u8mem[addr + i] = data[i];

  return addr;
}

/**
 * @param {WebAssembly.Instance} lcp
 * @param {number} ptr
 * @returns {Uint8Array | null}
 */
function bytesFromGo(lcp, ptr) {
  const { memory, bytesSize } = lcp.exports;
  const size = bytesSize(ptr);
  if (!size) return null;

  return new Uint8Array(memory.buffer).subarray(ptr, ptr + size);
}

/**
 * @param {WebAssembly.Instance} lcp
 * @param {File} file
 * @param {string} key
 */
async function decrypt(lcp, file, key) {
  const { decrypt, freeBytes } = lcp.exports;
  const fileData = new Uint8Array(await file.arrayBuffer());
  const goFile = newGoBytes(lcp, fileData);
  const goKey = newGoString(lcp, key);
  let goOut;

  try {
    goOut = decrypt(goFile, goKey);
    const decryptedData = bytesFromGo(lcp, goOut);
    if (!decryptedData) throw new Error("no decrypted data");
    const blob = new Blob([decryptedData], { type: "application/epub+zip" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `decrypted.${file.name}`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(link.href);
  } finally {
    freeBytes(goFile);
    freeBytes(goKey);
    if (goOut) freeBytes(goOut);
  }
}

async function main() {
  const go = new Go();
  const lcp = await getLCP(go);

  const submitButton = document.getElementById("button-submit");

  const resetSubmitButton = () => {
    submitButton.removeAttribute("disabled");
    submitButton.innerText = "Decrypt";
  };

  let busy = false;
  resetSubmitButton();

  document.getElementById("file-form").addEventListener("submit", (ev) => {
    ev.preventDefault();
    const formData = new FormData(ev.target);
    const file = formData.get("file");
    const key = formData.get("key");

    if (
      busy ||
      !file ||
      !(file instanceof File) ||
      !key ||
      typeof key !== "string"
    )
      return;

    busy = true;
    submitButton.setAttribute("disabled", "1");
    submitButton.innerText = "Decrypting...";

    decrypt(lcp, file, key)
      .catch((error) => {
        console.error(error);
        alert("There was an error decrypting the file");
      })
      .finally(() => {
        busy = false;
        resetSubmitButton();
      });
  });
}

main();
