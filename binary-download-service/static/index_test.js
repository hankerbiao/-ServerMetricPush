const fs = require("fs");
const path = require("path");
const vm = require("vm");

class Element {
  constructor(id = "") {
    this.id = id;
    this.innerHTML = "";
    this.listeners = {};
    this.dataset = {};
    this.classList = {
      add() {},
      remove() {}
    };
  }

  addEventListener(type, handler) {
    if (!this.listeners[type]) {
      this.listeners[type] = [];
    }
    this.listeners[type].push(handler);
  }

  querySelectorAll() {
    return [];
  }

  appendChild() {}

  remove() {}

  click() {}
}

function extractScript(html) {
  const match = html.match(/<script>([\s\S]*)<\/script>\s*<\/body>/);
  if (!match) {
    throw new Error("failed to extract inline script");
  }
  return match[1];
}

async function main() {
  const htmlPath = path.join(__dirname, "index.html");
  const html = fs.readFileSync(htmlPath, "utf8");
  const script = extractScript(html);

  const elements = new Map();
  const document = {
    getElementById(id) {
      if (!elements.has(id)) {
        elements.set(id, new Element(id));
      }
      return elements.get(id);
    },
    createElement() {
      return new Element();
    }
  };

  const context = vm.createContext({
    console,
    document,
    window: { location: { origin: "http://localhost:8080" } },
    navigator: { clipboard: { writeText: async () => {} } },
    fetch: async () => ({
      ok: true,
      async json() {
        return { files: [] };
      }
    }),
    setTimeout,
    clearTimeout,
    confirm: () => true,
    Date
  });

  vm.runInContext(script, context, { filename: htmlPath });
  await new Promise((resolve) => setImmediate(resolve));

  const content = document.getElementById("content");
  const initialListenerCount = (content.listeners.click || []).length;

  vm.runInContext("renderContent()", context);

  const listenerCountAfterRerender = (content.listeners.click || []).length;

  if (initialListenerCount !== 1) {
    throw new Error(`expected one initial content click listener, got ${initialListenerCount}`);
  }

  if (listenerCountAfterRerender !== 1) {
    throw new Error(`expected one content click listener after rerender, got ${listenerCountAfterRerender}`);
  }

  console.log("index frontend tests passed");
}

main().catch((error) => {
  console.error(error.message);
  process.exit(1);
});
