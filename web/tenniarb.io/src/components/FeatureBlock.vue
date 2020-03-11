<script lang="ts">
import Vue, { VueConstructor } from "vue";
import Component from "vue-class-component";
import { Prop } from "vue-property-decorator";

@Component({
  components: {}
})
export default class FeatureBlock extends Vue {
  @Prop({ required: true, type: String })
  title!: string;

  @Prop({ required: false, type: String, default: "white" })
  color!: string;
}
</script>

<template>
  <div class="feature-block" :class="color">
    <div class="table">
      <div class="tr">
        <div class="title">
          {{title}}
          <div class="line" />
        </div>
        <div class="td">
          <div class="descr">
            <slot name="descr" />
          </div>
          <div class="columns">
            <slot name="items" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
.feature-block {
  padding: 1em;
  &.grey {
    background-color: #f5f5f5 !important;
    color: #4a4a4a;
  }
  &.white {
    background-color: white;
  }
  .columns {
    padding-top: 1em;
    column-count: 2;
    vertical-align: middle;
    div {
      padding: 1em;
      font-size: 1.25rem !important;
    }
    div:before {
      content: "";
      padding: 0 !important;
      display: inline-block;
      vertical-align: middle;
      margin-right: 0.5em;
      background-image: url("../assets/check_mark.svg");
      width: 24px !important;
      height: 24px !important;
    }
  }
  .table {
    display: table;
    table-layout: fixed;
    width: 100%;
    height: 10em;
    padding: 2em;
    .tr {
      padding: 2em;
      display: table-row;
    }
    .td {
      width: 95%;
    }
    .title {
      //   border: 1px solid;
      width: 30%;
      vertical-align: top;
      display: table-cell;
      text-align: center;
      font-size: 1.5rem;
      color: #363636;
      .line {
        height: 0.1em;
        background-color: #f34b13;
        position: relative;
        left: 25%;
        width: 50%;
      }
    }
    .descr {
      //   border: 1px solid;
      //   padding: 2em;
      font-size: 1.25rem !important;
    }
  }
}
</style>
