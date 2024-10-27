// Prevent HTML elements with the .overscroll-contain-force class from leaking
// scroll events to their parent.
window.addEventListener('load', function () {
    const elements = document.getElementsByClassName('overscroll-contain-force');
    const listener = function (evt) {
        // Only prevent bubbling-up the scroll event when the element doesn't have a scrollbar.
        // When it does have a scrollbar, let the CSS overscroll-behavior property handle it.
        if (this.scrollHeight > this.clientHeight) {
            return;
        }
        evt.preventDefault();
    };
    for (let element of elements) {
        element.addEventListener('wheel', listener, {passive: false});
        element.addEventListener('touchmove', listener, {passive: false});
    }
});

function toggleMenu() {
    const nav = document.getElementById('sidebar-nav');
    nav.dataset.open = nav.dataset.open === 'open' ? 'closed' : 'open';
}