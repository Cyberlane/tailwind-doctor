const header = document.querySelector("[data-header]");
const navigation = document.querySelector("[data-nav]");
const navigationToggle = document.querySelector("[data-nav-toggle]");

const updateHeader = () => header?.classList.toggle("scrolled", window.scrollY > 20);
updateHeader();
window.addEventListener("scroll", updateHeader, { passive: true });

navigationToggle?.addEventListener("click", () => {
  const isOpen = navigation?.classList.toggle("open") ?? false;
  navigationToggle.setAttribute("aria-expanded", String(isOpen));
});

navigation?.addEventListener("click", (event) => {
  if (!(event.target instanceof HTMLAnchorElement)) return;
  navigation.classList.remove("open");
  navigationToggle?.setAttribute("aria-expanded", "false");
});

const copyText = async (text) => {
  await navigator.clipboard.writeText(text);
};

document.querySelectorAll("[data-command]").forEach((command) => {
  const button = command.querySelector("[data-copy]");
  const label = command.querySelector("[data-copy-label]");

  button?.addEventListener("click", async () => {
    try {
      await copyText("npx tw-doctor .");
      command.classList.add("copied");
      if (label) label.textContent = "Copied";
      window.setTimeout(() => {
        command.classList.remove("copied");
        if (label) label.textContent = "Copy";
      }, 1600);
    } catch {
      if (label) label.textContent = "Select text";
    }
  });
});

document.querySelectorAll("[data-copy-text]").forEach((button) => {
  button.addEventListener("click", async () => {
    const originalLabel = button.textContent;
    try {
      await copyText(button.getAttribute("data-copy-text") ?? "");
      button.textContent = "Copied";
      window.setTimeout(() => { button.textContent = originalLabel; }, 1600);
    } catch {
      button.textContent = "Select";
    }
  });
});

const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
const reveals = document.querySelectorAll(".reveal");

if (reduceMotion || !("IntersectionObserver" in window)) {
  reveals.forEach((element) => element.classList.add("visible"));
} else {
  const observer = new IntersectionObserver((entries) => {
    entries.forEach((entry) => {
      if (!entry.isIntersecting) return;
      entry.target.classList.add("visible");
      observer.unobserve(entry.target);
    });
  }, { threshold: 0.12 });

  reveals.forEach((element) => observer.observe(element));
}
