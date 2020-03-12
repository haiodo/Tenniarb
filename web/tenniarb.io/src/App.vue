<template>
  <div class="app">
    <div class="navbar">
      <div class="nav">
        <router-link class="nav-item" to="/">Features</router-link>|
        <router-link class="nav-item" to="/blog">Blog</router-link>|
        <router-link class="nav-item" to="/docs">Documentation</router-link>|
        <router-link class="nav-item" to="/downloads">Download</router-link>|
        <router-link class="nav-item" to="/about">About</router-link>
      </div>
    </div>
    <div class="container">
      <router-view />
      <div class="footer">Copyright © 2020 The Tenniarb app authors</div>
    </div>
  </div>
</template>

<style scoped lang="scss">
.app {
  font-family: Avenir, Helvetica, Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  color: #2c3e50;
  margin: 0;
  padding: 0;
}

.navbar {
  left: 0;
  top: 0;
  position: fixed;
  right: 0;
  z-index: 30;
  min-height: 4rem;
  text-align: right;
  background-color: #2c3e50;
  vertical-align: middle;
  .nav {
    padding: 1em;
    .nav-item {
      font-weight: bold;
      color: grey;
      margin-top: 1em;

      &.router-link-exact-active {
        color: #f34b13;
      }
    }
  }
}
.container {
  top: 3.5rem;
  position: relative;
  margin: -0.5rem;
  left: 0;
  right: 0;
  overflow-x: hidden;
  overflow-y: auto;
  height: 100%;
}
.footer {
  text-align: center;
  background-color: #f5f5f5;
  padding: 5em;
}
</style>
