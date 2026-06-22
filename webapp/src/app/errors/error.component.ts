import { Component, OnInit, ChangeDetectionStrategy } from '@angular/core';

@Component({
    selector: 'app-error',
    templateUrl: './error.component.html',
    styleUrls: ['./error.component.css'],
    changeDetection: ChangeDetectionStrategy.Eager,
    standalone: false
})
export class ErrorComponent implements OnInit {

  constructor() { }

  ngOnInit() {
  }

}
