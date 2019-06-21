import { Component, OnInit, ViewChild, Output, EventEmitter } from '@angular/core';
import { NgbModal, ModalDismissReasons } from '@ng-bootstrap/ng-bootstrap';
import { ServiceDetail, ServiceDetailService }  from './service-detail.service';

import * as moment from 'moment';

@Component({
  selector: 'app-service-detail-confirm',
  templateUrl: './confirm.component.html',
})
export class ConfirmChildComponent implements OnInit {

  closeResult: string;
  selectedItem: string;
  loading: boolean = false;
  serviceName: string;
  confirmType: string;

  @ViewChild('confirm', { static: true }) confirmModal : NgbModal;

  @Output() deletedItem: EventEmitter<any> = new EventEmitter<any>();
  @Output() deletingItem: EventEmitter<any> = new EventEmitter<any>();


  constructor(
    private modalService: NgbModal,
    private sds: ServiceDetailService
  ) {}

  ngOnInit(): void { 

  }

  open(action, confirmType, serviceName, selectedItem) {
    if(!selectedItem) {
      return
    }
    this.loading = true
    this.serviceName = serviceName
    this.selectedItem = selectedItem
    this.confirmType = confirmType
    this.modalService.open(this.confirmModal, { windowClass: 'confirm-modal' } ).result.then((result) => {
      this.closeResult = `Closed with: ${result}`;
      if(result == "DeleteItem") {
        if(action == 'deleteParameter') {
          this.loading = true
          this.deletingItem.emit(true)
          this.sds.deleteParameter(this.serviceName, selectedItem).subscribe(data => {
            this.loading = false
            this.deletingItem.emit(false)
            this.deletedItem.emit({ "action": action, "selectedItem": selectedItem })
          })
        } else if(action == 'deleteAutoscalingPolicy') {
          this.loading = true
          this.deletingItem.emit(true)
          this.sds.deleteAutoscalingPolicy(this.serviceName, selectedItem).subscribe(data => {
            this.loading = false
            this.deletingItem.emit(false)
            this.deletedItem.emit({ "action": action, "selectedItem": selectedItem })
          })
        } else if(action == 'disableAutoscaling') {
          this.loading = true
          this.deletingItem.emit(true)
          this.sds.disableAutoscaling(this.serviceName).subscribe(data => {
            this.loading = false
            this.deletingItem.emit(false)
            this.deletedItem.emit({ "action": action, "selectedItem": "" })
          })
        }
      }
    }, (reason) => {
      this.closeResult = `Dismissed ${this.getDismissReason(reason)}`;
    });
  }
  private getDismissReason(reason: any): string {
    if (reason === ModalDismissReasons.ESC) {
      return 'by pressing ESC';
    } else if (reason === ModalDismissReasons.BACKDROP_CLICK) {
      return 'by clicking on a backdrop';
    } else {
      return  `with: ${reason}`;
    }
  }
}
