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
  selectedParameter: string;
  loading: boolean = false;
  serviceName: string;

  @ViewChild('confirm') confirmModal : NgbModal;

  @Output() deletedParameter: EventEmitter<any> = new EventEmitter<any>();
  @Output() deletingParameter: EventEmitter<any> = new EventEmitter<any>();

  constructor(
    private modalService: NgbModal,
    private sds: ServiceDetailService
  ) {}

  ngOnInit(): void { 

  }

  open(action, serviceName, selectedParameter) {
    if(!selectedParameter) {
      return
    }
    this.loading = true
    this.serviceName = serviceName
    this.selectedParameter = selectedParameter
    this.modalService.open(this.confirmModal, { windowClass: 'confirm-modal' } ).result.then((result) => {
      this.closeResult = `Closed with: ${result}`;
      if(result == "DeleteParameter") {
        this.loading = true
        this.deletingParameter.emit(true)
        this.sds.deleteParameter(this.serviceName, selectedParameter).subscribe(data => {
          this.loading = false
          this.deletingParameter.emit(false)
          this.deletedParameter.emit(selectedParameter)
        })
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
