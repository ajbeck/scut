document.querySelectorAll("[data-copy-value]").forEach((button) => {
  button.addEventListener("click", async () => {
    const value = button.getAttribute("data-copy-value");
    if (!value) return;
    await navigator.clipboard.writeText(value);
    const original = button.textContent;
    button.textContent = "Copied";
    window.setTimeout(() => {
      button.textContent = original;
    }, 1200);
  });
});
