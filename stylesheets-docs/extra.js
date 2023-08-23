// Open external links in new tab
var links = document.links;

for(var i = 0; i < links.length; i++) {
  if (links[i].hostname != window.location.hostname) {
    links[i].target = '_blank';
    links[i].rel = 'noopener';
  }
}

function injectScript(src, cb) {
  let script = document.createElement('script');

  script.src = src;
  cb && (script.onload = cb);
  document.body.append(script);
}

// Collapse expanded menu items when a new item is expanded
var navClassName = ".md-nav__toggle";
var navigationElements = document.querySelectorAll(navClassName);

function getAllNavigationElements(element, selector){
  if(element.parentElement && element.parentElement.parentElement && element.parentElement.parentElement.children){
    var allChildren = element.parentElement.parentElement.children;
    for (let index = 0; index < allChildren.length; index++) {
      var child = allChildren[index];
      var navigationInput = child.querySelector(selector);
      if(navigationInput && navigationInput !== element){
        navigationInput.checked = false;
      }
    }
  }
}

navigationElements.forEach(el => {
  el.addEventListener('change', function(){
    getAllNavigationElements(this, navClassName);
  }, false);
})

// Highlight the search string if URL contains ?search

const urlParams = new URLSearchParams(window.location.search);
const myParam = urlParams.get('search');
var searchBar = document.getElementsByClassName('md-search__input');

if(myParam !== null) {
  document.getElementById("__search").checked = true;
}
