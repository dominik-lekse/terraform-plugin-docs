let fetchContent = (u) => {
    fetch("/markdown/" + u)
        .then(r => r.json())
        .then(r => {
            document.getElementById("ember21").value = r.content // TODO shouldn't bake in the ID
            document.getElementById("ember21").dispatchEvent(new Event("input", {"bubbles": true}))
        })

    document.getElementById("ember21").style.display = "none"
}

let updateMenu = (n) => {
    fetch("/markdown/menu")
        .then(r => r.text())
        .then(r => {
            n.innerHTML = r
        })
}

let textArea = new MutationObserver((mutations, ob) => {
    mutations.forEach((mutation) => {
        if (!mutation.addedNodes) return

        for (let i = 0; i < mutation.addedNodes.length; i++) {
            // do things to your newly added nodes here
            let node = mutation.addedNodes[i]
            if (node.nodeName === "TEXTAREA") { // TODO is this smart enough?
                fetchContent("docs/index.md") // TODO this shouldn't be hard coded
                ob.disconnect()
            }
        }
    })
})
let menu = new MutationObserver((mutations, ob) => {
    mutations.forEach((mutation) => {
        if (!mutation.addedNodes) return

        for (let i = 0; i < mutation.addedNodes.length; i++) {
            // do things to your newly added nodes here
            let node = mutation.addedNodes[i]
            if (node.nodeName === "DIV" && node.getAttribute("class") === "provider-docs-menu") {
                updateMenu(node)
                ob.disconnect()
            }
        }
    })
})

textArea.observe(document.body, {
    childList: true
    , subtree: true
    , attributes: false
    , characterData: false
})
menu.observe(document.body, {
    childList: true
    , subtree: true
    , attributes: false
    , characterData: false
})